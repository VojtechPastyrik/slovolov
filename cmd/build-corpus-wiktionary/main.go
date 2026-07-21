// build-corpus-wiktionary produces a clean Czech noun corpus from Kaikki.org's
// export of the Czech Wiktionary. Compared to the UDPipe-on-subtitles pipeline,
// this source is human-curated so it doesn't pull in TV character names or
// English words.
//
// Expected input file: JSONL from https://kaikki.org/dictionary/Czech/
// One entry per line, at least these fields used:
//
//   {
//     "word": "pes",
//     "pos": "noun",           // "name" is used for proper nouns — skipped
//     "lang": "Czech",
//     "senses": [{ "tags": ["masculine", "animate"] , "glosses": [...] }],
//     "head_templates": [{ "name": "cs-noun", ... }]
//   }
//
// Filters:
//   - pos == "noun" (excludes "name" / "proper noun" / other POS)
//   - lang == "Czech"
//   - not multi-word (no whitespace)
//   - senses don't tag it as verbal noun / proper / archaic / obsolete
//   - passes basic lemma sanity (letters only, >= 3 chars)
//
// Optionally the output is ordered by an external frequency list so that
// common words end up first (nice for the game when the corpus is trimmed).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"
)

type kaikkiEntry struct {
	Word          string        `json:"word"`
	POS           string        `json:"pos"`
	Lang          string        `json:"lang"`
	LangCode      string        `json:"lang_code"`
	Senses        []kaikkiSense `json:"senses"`
	HeadTemplates []kaikkiTpl   `json:"head_templates"`
	RawTags       []string      `json:"raw_tags"`
}

type kaikkiSense struct {
	Tags    []string `json:"tags"`
	RawTags []string `json:"raw_tags"`
	Glosses []string `json:"glosses"`
}

type kaikkiTpl struct {
	Name string `json:"name"`
}

// senseTagBlockers marks a sense we don't want to keep. Only tags that
// indicate the entry itself is not a plain common noun. Sense-level style
// tags like "slang" or "vulgar" are NOT here — a word like "pes" has
// several slang senses but is obviously a valid common noun; blocking on
// those would nuke it.
var senseTagBlockers = map[string]struct{}{
	"form-of":           {},
	"alt-of":            {},
	"proper":            {},
	"proper-noun":       {},
	"verbal-noun":       {},
	"abbreviation":      {},
	"initialism":        {},
	"acronym":           {},
	"letter":            {},
	"symbol":            {},
	"name":              {},
	"surname":           {},
	"given-name":        {},
	"male-given-name":   {},
	"female-given-name": {},
	"place":             {},
	"toponym":           {},
	"demonym":           {},
}

// verbalNounSuffixes marks Czech verbal-noun (substantivum verbální) endings.
// These are grammatically nouns but derived from verbs — game-unfriendly
// ("myšlení", "vraždění", "plavání"). If the user really wants a specific
// one, put it in corpus/cs-supplement.txt.
var verbalNounSuffixes = []string{"ání", "ení", "ění", "tí"}

// rawTagBlockers are Czech-language markers Kaikki writes into `raw_tags`
// (the pass-through fields taken from the source Wiktionary templates,
// not the normalized English tags). They mark entries we don't want.
var rawTagBlockers = map[string]struct{}{
	"vlastní":    {}, // "vlastní jméno" — proper name
	"jméno":      {},
	"příjmení":   {}, // surname
	"přezdívka":  {}, // nickname
	"archaický":  {},
	"archaicky":  {},
	"zastaralý":  {},
	"zastarale":  {},
	"knižně":     {}, // literary/bookish
	"nářeční":    {},
	"nářečně":    {},
	"hanlivě":    {}, // pejorative
	"vulgárně":   {},
	"zhruběle":   {},
	"expresivně": {},
	"onomatopoické": {},
	"zkratka":    {},
	"iniciálová zkratka": {},
}

// glossBlockerSubstrings triggers if any sense's gloss contains one of
// these Czech phrases (case-folded). Catches proper names that lack the
// raw_tag but describe themselves as "křestní jméno" etc. in the gloss.
var glossBlockerSubstrings = []string{
	"křestní jméno",
	"mužské křestní jméno",
	"ženské křestní jméno",
	"příjmení",
	"jméno rodu",
	"jméno města",
	"jméno obce",
	"jméno postavy",
	"vlastní jméno",
	"personifikace",
}

// posBlockers rejects any Kaikki `pos` value that isn't a plain common
// noun. Kaikki records proper nouns as "name".
var posBlockers = map[string]struct{}{
	"name":        {},
	"proper noun": {},
	"proper-noun": {},
}

func main() {
	var (
		inPath       = flag.String("in", "corpus/kaikki-cs.jsonl", "Kaikki JSONL for Czech (from https://kaikki.org/dictionary/Czech/)")
		freqPath     = flag.String("frequency", "corpus/cs_50k.txt", "frequency list — when set, output is restricted to lemmas that appear here (avoids obscure Wiktionary tail words)")
		keepUnranked = flag.Bool("keep-unranked", false, "also emit Wiktionary lemmas that do not appear in the frequency list (huge, includes archaic/regional words)")
		outPath      = flag.String("out", "corpus/cs.txt", "output path — one noun lemma per line")
		supplement   = flag.String("supplement", "corpus/cs-supplement.txt", "extra lemmas to include (optional, missing file is OK)")
		blockPath    = flag.String("block", "corpus/cs-block.txt", "lemmas to strip (optional, missing file is OK)")
	)
	flag.Parse()

	block, err := loadOptionalSet(*blockPath)
	if err != nil {
		log.Fatalf("read block list: %v", err)
	}
	log.Printf("block list: %d entries", len(block))

	nouns, err := parseKaikki(*inPath)
	if err != nil {
		log.Fatalf("parse kaikki: %v", err)
	}
	log.Printf("wiktionary nouns after filter: %d", len(nouns))

	extras, err := loadOptionalList(*supplement)
	if err != nil {
		log.Fatalf("read supplement: %v", err)
	}
	for _, w := range extras {
		nouns[w] = struct{}{}
	}
	log.Printf("supplement added: %d entries (total distinct: %d)", len(extras), len(nouns))

	// Apply the block list *after* everything else — user's manual veto.
	for w := range block {
		delete(nouns, w)
	}

	ordered, err := orderByFrequency(nouns, *freqPath, *keepUnranked)
	if err != nil {
		log.Fatalf("order by frequency: %v", err)
	}
	log.Printf("final size: %d", len(ordered))

	if err := writeWords(*outPath, ordered); err != nil {
		log.Fatalf("write output: %v", err)
	}
	log.Printf("wrote %s: %d nouns", *outPath, len(ordered))
}

// parseKaikki streams the JSONL file and returns a set of lemmas that
// pass all filters.
func parseKaikki(path string) (map[string]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 4*1024*1024), 32*1024*1024)

	seen, kept, entries := 0, 0, 0
	for sc.Scan() {
		entries++
		line := sc.Bytes()
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e kaikkiEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue // one bad line shouldn't kill the whole pass
		}
		// Kaikki uses the native language name ("čeština") in `lang`; rely on
		// `lang_code` as the authoritative filter.
		if e.LangCode != "cs" {
			continue
		}
		if _, blocked := posBlockers[e.POS]; blocked {
			continue
		}
		if e.POS != "noun" {
			continue
		}
		if hasBlockedRawTag(e.RawTags) {
			continue
		}
		if !hasKeepableSense(e.Senses) {
			continue
		}
		if hasVerbalNounSuffix(e.Word) {
			continue
		}
		if hasProperHeadTemplate(e.HeadTemplates) {
			continue
		}
		lemma := strings.TrimSpace(e.Word)
		if strings.ContainsAny(lemma, " \t") {
			continue // multi-word entry
		}
		lemma = strings.ToLower(lemma)
		if !isPlausibleLemma(lemma) {
			continue
		}
		seen++
		if _, dup := out[lemma]; dup {
			continue
		}
		out[lemma] = struct{}{}
		kept++
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan kaikki: %w", err)
	}
	log.Printf("kaikki: read %d entries, %d matched, %d unique kept", entries, seen, kept)
	return out, nil
}

// hasKeepableSense returns true if at least one sense of an entry lacks
// blocker markers (English tags, Czech raw_tags, or blocker phrases in
// the gloss). Some legitimate common nouns carry a form-of or slang
// sense alongside the primary meaning; those are still valid words.
func hasKeepableSense(senses []kaikkiSense) bool {
	if len(senses) == 0 {
		return true
	}
	for _, s := range senses {
		if senseBlocked(s) {
			continue
		}
		return true
	}
	return false
}

func senseBlocked(s kaikkiSense) bool {
	for _, tag := range s.Tags {
		if _, bad := senseTagBlockers[tag]; bad {
			return true
		}
	}
	for _, tag := range s.RawTags {
		if _, bad := rawTagBlockers[strings.ToLower(tag)]; bad {
			return true
		}
	}
	for _, gloss := range s.Glosses {
		lower := strings.ToLower(gloss)
		for _, sub := range glossBlockerSubstrings {
			if strings.Contains(lower, sub) {
				return true
			}
		}
	}
	return false
}

func hasBlockedRawTag(rawTags []string) bool {
	for _, tag := range rawTags {
		if _, bad := rawTagBlockers[strings.ToLower(tag)]; bad {
			return true
		}
	}
	return false
}

func hasVerbalNounSuffix(word string) bool {
	lower := strings.ToLower(word)
	for _, suf := range verbalNounSuffixes {
		if strings.HasSuffix(lower, suf) {
			return true
		}
	}
	return false
}

func hasProperHeadTemplate(tpls []kaikkiTpl) bool {
	for _, t := range tpls {
		lower := strings.ToLower(t.Name)
		if strings.Contains(lower, "proper") || strings.Contains(lower, "name") {
			return true
		}
	}
	return false
}

// orderByFrequency returns lemmas ordered by descending frequency. Unless
// keepUnranked is true, Wiktionary lemmas missing from the frequency list
// are dropped — the intent is that only words common enough to appear in
// everyday Czech (present in the subtitle-derived frequency list) survive.
func orderByFrequency(nouns map[string]struct{}, freqPath string, keepUnranked bool) ([]string, error) {
	var ordered []string
	used := make(map[string]struct{}, len(nouns))

	if freqPath != "" {
		f, err := os.Open(freqPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			defer f.Close()
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
				if _, isNoun := nouns[line]; !isNoun {
					continue
				}
				if _, dup := used[line]; dup {
					continue
				}
				used[line] = struct{}{}
				ordered = append(ordered, line)
			}
			if err := sc.Err(); err != nil {
				return nil, err
			}
		}
	}

	if !keepUnranked {
		return ordered, nil
	}

	// Append remaining unmatched lemmas (Wiktionary noun that wasn't in
	// the frequency list) — for callers that want the complete Wiktionary
	// vocabulary regardless of usage frequency.
	var rest []string
	for w := range nouns {
		if _, already := used[w]; already {
			continue
		}
		rest = append(rest, w)
	}
	sortStrings(rest)
	return append(ordered, rest...), nil
}

func sortStrings(s []string) {
	// tiny inline sort to avoid an extra import
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
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
