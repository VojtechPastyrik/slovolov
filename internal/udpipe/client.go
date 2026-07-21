// Package udpipe wraps the LINDAT UDPipe REST API used by the corpus filter.
package udpipe

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://lindat.mff.cuni.cz/services/udpipe/api"

// DefaultCzechModel is a currently-served Czech UD 2.x model.
// See: https://lindat.mff.cuni.cz/services/udpipe/api/models
const DefaultCzechModel = "czech-pdt-ud-2.15-241121"

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewClient(model string) *Client {
	if model == "" {
		model = DefaultCzechModel
	}
	return &Client{
		baseURL: defaultBaseURL,
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
	}
}

type processResponse struct {
	Result string `json:"result"`
	Model  string `json:"model"`
}

// Token is one parsed row from UDPipe's CoNLL-U output.
type Token struct {
	Form  string
	Lemma string
	UPOS  string
	Feats map[string]string
}

// Analyze feeds `words` to UDPipe (one per line) and returns the parsed
// tokens in input order. Tokens whose form doesn't match any input word
// (rare — mostly whitespace artefacts) are silently dropped.
func (c *Client) Analyze(ctx context.Context, words []string) ([]Token, error) {
	if len(words) == 0 {
		return nil, nil
	}
	form := url.Values{}
	form.Set("model", c.model)
	form.Set("tokenizer", "")
	form.Set("tagger", "")
	form.Set("data", strings.Join(words, "\n"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/process", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("udpipe status %d: %s", resp.StatusCode, snippet(body))
	}

	var parsed processResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode udpipe response: %w", err)
	}
	tokens, err := parseCoNLLU(parsed.Result)
	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func snippet(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

// parseCoNLLU extracts NOUN-relevant fields from CoNLL-U output. Comment
// lines and multiword-token ranges are skipped.
func parseCoNLLU(s string) ([]Token, error) {
	var out []Token
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 6 {
			continue
		}
		// Skip multiword token ranges like "1-2".
		if strings.Contains(cols[0], "-") || strings.Contains(cols[0], ".") {
			continue
		}
		out = append(out, Token{
			Form:  cols[1],
			Lemma: cols[2],
			UPOS:  cols[3],
			Feats: parseFeats(cols[5]),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, errors.New("read conllu: " + err.Error())
	}
	return out, nil
}

func parseFeats(s string) map[string]string {
	if s == "" || s == "_" {
		return nil
	}
	out := make(map[string]string)
	for _, pair := range strings.Split(s, "|") {
		if eq := strings.IndexByte(pair, '='); eq > 0 {
			out[pair[:eq]] = pair[eq+1:]
		}
	}
	return out
}
