// build-corpus filters a raw Czech frequency word list down to nouns in
// singular nominative, deduplicating by lemma. It uses UDPipe (LINDAT) for
// morphological analysis so the runtime image never has to ship an NLP
// model.
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
		inPath  = flag.String("in", "corpus/cs_50k.txt", "raw frequency list (one word per line, extra columns ignored)")
		outPath = flag.String("out", "corpus/cs.txt", "output path — one noun lemma per line, frequency order")
		model   = flag.String("model", udpipe.DefaultCzechModel, "UDPipe model name")
		batch   = flag.Int("batch", 500, "words per UDPipe request")
		limit   = flag.Int("limit", 0, "cap on input words (0 = all)")
	)
	flag.Parse()

	raw, err := readWords(*inPath, *limit)
	if err != nil {
		log.Fatalf("read input: %v", err)
	}
	log.Printf("input words after basic filter: %d", len(raw))

	client := udpipe.NewClient(*model)
	ctx := context.Background()

	seen := make(map[string]struct{})
	var out []string

	for start := 0; start < len(raw); start += *batch {
		end := start + *batch
		if end > len(raw) {
			end = len(raw)
		}
		reqCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		tokens, err := client.Analyze(reqCtx, raw[start:end])
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
			if _, dup := seen[lemma]; dup {
				continue
			}
			seen[lemma] = struct{}{}
			out = append(out, lemma)
			kept++
		}
		log.Printf("progress %d/%d (kept %d, total %d)", end, len(raw), kept, len(out))
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
