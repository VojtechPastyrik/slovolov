// build-corpus filters a raw Czech frequency word list down to nouns in
// singular nominative, deduplicating by lemma. It uses UDPipe (LINDAT) for
// morphological analysis so the runtime image never has to ship an NLP
// model.
//
// UDPipe tagging on isolated words (no sentence context) misclassifies some
// proper names as NOUN. To compensate, this tool merges in two optional
// hand-curated lists next to the frequency source:
//
//   corpus/cs-supplement.txt — extra lemmas to include (concrete nouns
//                              that a subtitle-frequency list would miss)
//   corpus/cs-block.txt      — lemmas to strip (common first names,
//                              English words that leaked into the source)
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/vojtechpastyrik/slovolov/internal/udpipe"
)

func main() {
	var (
		inPath      = flag.String("in", "corpus/cs_50k.txt", "raw frequency list (one word per line, extra columns ignored)")
		supplement  = flag.String("supplement", "corpus/cs-supplement.txt", "extra lemmas to include (optional, missing file is OK)")
		blockPath   = flag.String("block", "corpus/cs-block.txt", "lemmas to strip after tagging (optional, missing file is OK)")
		outPath     = flag.String("out", "corpus/cs.txt", "output path — one noun lemma per line")
		model       = flag.String("model", udpipe.DefaultCzechModel, "UDPipe model name")
		batch       = flag.Int("batch", 500, "words per UDPipe request")
		limit       = flag.Int("limit", 0, "cap on input words (0 = all)")
	)
	flag.Parse()

	block, err := loadOptionalSet(*blockPath)
	if err != nil {
		log.Fatalf("read block list: %v", err)
	}
	log.Printf("block list: %d entries", len(block))

	extras, err := loadOptionalList(*supplement)
	if err != nil {
		log.Fatalf("read supplement: %v", err)
	}
	log.Printf("supplement: %d entries", len(extras))

	raw, err := readWords(*inPath, *limit)
	if err != nil {
		log.Fatalf("read input: %v", err)
	}
	log.Printf("input words after basic filter: %d", len(raw))

	// Merge supplement into the front so those lemmas always survive even
	// if UDPipe would mistag them.
	merged := append(append([]string(nil), extras...), raw...)

	client := udpipe.NewClient(*model)
	ctx := context.Background()

	seen := make(map[string]struct{})
	var out []string

	for start := 0; start < len(merged); start += *batch {
		end := start + *batch
		if end > len(merged) {
			end = len(merged)
		}
		reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		tokens, err := client.Analyze(reqCtx, merged[start:end])
		cancel()
		if err != nil {
			log.Fatalf("udpipe batch [%d:%d]: %v", start, end, err)
		}
		kept := 0
		for _, tok := range tokens {
			if !isSingularNounNom(tok) {
				continue
			}
			lemma := strings.ToLower(strings.TrimSpace(tok.Lemma))
			if !isPlausibleLemma(lemma) {
				continue
			}
			if _, blocked := block[lemma]; blocked {
				continue
			}
			if _, dup := seen[lemma]; dup {
				continue
			}
			seen[lemma] = struct{}{}
			out = append(out, lemma)
			kept++
		}
		log.Printf("progress %d/%d (kept %d, total %d)", end, len(merged), kept, len(out))
	}

	if err := writeWords(*outPath, out); err != nil {
		log.Fatalf("write output: %v", err)
	}
	log.Printf("wrote %s: %d nouns", *outPath, len(out))
}

func isSingularNounNom(t udpipe.Token) bool {
	if t.UPOS != "NOUN" {
		return false
	}
	if t.Feats["Number"] != "Sing" {
		return false
	}
	if t.Feats["Case"] != "Nom" {
		return false
	}
	// UDPipe sometimes flags a token as NOUN but still fills NameType
	// (Prs = person, Geo = geographic, Com = company). Those are proper
	// names dressed up as common nouns — drop them.
	if _, isName := t.Feats["NameType"]; isName {
		return false
	}
	// Filter Czech verbal nouns (substantiva verbální) like "vraždění",
	// "plavání", "myšlení" — grammatically nouns, but derived from verbs
	// and awkward to guess. UDPipe marks them with VerbForm=Vnoun or by
	// carrying an Aspect feat (only verbs and their derivations have it).
	if t.Feats["VerbForm"] == "Vnoun" {
		return false
	}
	if _, hasAspect := t.Feats["Aspect"]; hasAspect {
		return false
	}
	return true
}

func isPlausibleLemma(w string) bool {
	if len([]rune(w)) < 3 {
		return false
	}
	for _, r := range w {
		if !unicode.IsLetter(r) && r != '-' {
			return false
		}
	}
	return true
}

func readWords(path string, limit int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if idx := strings.IndexByte(line, ' '); idx > 0 {
			line = line[:idx]
		}
		line = strings.ToLower(line)
		if !isPlausibleLemma(line) {
			continue
		}
		out = append(out, line)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, sc.Err()
}

// loadOptionalList reads a plain word-per-line file. Missing file is not
// an error — returns an empty slice.
func loadOptionalList(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.ToLower(line)
		if !isPlausibleLemma(line) {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
}

func loadOptionalSet(path string) (map[string]struct{}, error) {
	words, err := loadOptionalList(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(words))
	for _, w := range words {
		out[w] = struct{}{}
	}
	return out, nil
}

func writeWords(path string, words []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, word := range words {
		if _, err := fmt.Fprintln(w, word); err != nil {
			return err
		}
	}
	return w.Flush()
}
