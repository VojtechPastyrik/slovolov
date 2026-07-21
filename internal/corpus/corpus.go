package corpus

import (
	"bufio"
	"os"
	"strings"
	"unicode"

	"github.com/vojtechpastyrik/slovolov/internal/game"
)

// Load parses a word-per-line corpus file (or `word count` frequency lines)
// and returns cleaned, deduplicated Czech words suitable for the game.
func Load(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := make(map[string]struct{})
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		word := line
		if idx := strings.IndexByte(word, ' '); idx > 0 {
			word = word[:idx]
		}
		word = game.Normalize(word)
		if !isValidWord(word) {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		out = append(out, word)
	}
	return out, sc.Err()
}

func isValidWord(w string) bool {
	if len([]rune(w)) < 3 {
		return false
	}
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
