// Package llm generates the daily secret word and its ranked word list, and
// scores guesses that are not on that list. It replaces the embedding-based
// similarity pipeline: Czech word association is judged by the model rather
// than by cosine distance between sentence-retrieval vectors.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrUnknownWord is returned when the model does not recognise a guess as a
// Czech word at all.
var ErrUnknownWord = errors.New("unknown word")

// The three jobs this package runs have very different shapes, so they do not
// share a model. Picking the secret is one call whose quality sets up the whole
// day, and it is worth the best model. Filling the bands is bulk listing across
// a dozen calls — the bulk of the bill. Scoring a guess rates a single word and
// runs once per unseen word per day, so it is the one that scales with players.
const (
	defaultModel     = "claude-opus-5"
	defaultBulkModel = "claude-sonnet-5"
	// Scoring a guess looks cheap enough for the smallest model, but it is not:
	// the score becomes a rank, and a noisy rating puts an unrelated word in the
	// top of the ranking. It is also cached per word per puzzle across all
	// players, so the call volume is bounded by distinct words guessed — the
	// saving from a smaller model here is pennies and the cost is correctness.
	defaultGuessModel = "claude-sonnet-5"
)

// Client talks to the Claude API.
type Client struct {
	api        anthropic.Client
	model      anthropic.Model
	bulkModel  anthropic.Model
	guessModel anthropic.Model
}

// NewFromEnv builds a client from ANTHROPIC_API_KEY (required) plus optional
// per-job model overrides: ANTHROPIC_MODEL picks the secret, ANTHROPIC_BULK_MODEL
// fills the ranking bands, ANTHROPIC_GUESS_MODEL scores player guesses.
func NewFromEnv() (*Client, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		return nil, errors.New("ANTHROPIC_API_KEY is not set")
	}
	pick := func(env, fallback string) anthropic.Model {
		if v := os.Getenv(env); v != "" {
			return anthropic.Model(v)
		}
		return anthropic.Model(fallback)
	}
	return &Client{
		api:        anthropic.NewClient(option.WithAPIKey(key)),
		model:      pick("ANTHROPIC_MODEL", defaultModel),
		bulkModel:  pick("ANTHROPIC_BULK_MODEL", defaultBulkModel),
		guessModel: pick("ANTHROPIC_GUESS_MODEL", defaultGuessModel),
	}, nil
}

// call is one request's budget: which model answers it, how much room it gets,
// and how much deliberation it is allowed. Thinking is on by default on the
// Opus tier, and it is billed at output rates — for jobs that only list or rate
// words it is pure cost, so those turn it off.
type call struct {
	model     anthropic.Model
	maxTokens int64
	// effort is left empty for models that reject the parameter (Haiku).
	effort   anthropic.OutputConfigEffort
	thinking bool
}

// ScoredWord is one entry of the daily ranking: a Czech word and how close it
// is to the secret on a 0-100 scale.
type ScoredWord struct {
	Word  string
	Score float64
}

// band describes one slice of the ranking: the score window it fills and how
// many words belong in it. The model never sees these numbers as something to
// emit — it only orders words, and the window turns that order into scores.
type band struct {
	low, high float64
	count     int
	guidance  string
}

// closestLow and closestHigh bound the band held by the words picked alongside
// the secret itself.
const (
	closestLow  = 90
	closestHigh = 99.9
)

// chunkSize caps how many words one call has to produce. Larger batches blow
// past max_tokens once thinking is counted in, and the reply comes back
// truncated with no parseable JSON.
const chunkSize = 200

// parallelChunks bounds how many generation calls are in flight at once.
const parallelChunks = 4

// domains spread the far bands across everyday areas of life. Chunks run in
// parallel and cannot see each other's output, so without a thematic split
// they independently reach for the same handful of common words and most of
// the batch is thrown away as duplicates.
var domains = []string{
	"jídlo a pití",
	"kuchyně a domácnost",
	"oblečení a móda",
	"tělo, zdraví a nemoci",
	"příroda, krajina a počasí",
	"zvířata",
	"rostliny a zahrada",
	"doprava a cestování",
	"město, budovy a bydlení",
	"práce, řemesla a nářadí",
	"škola, úřady a peníze",
	"sport, hry a zábava",
	"hudba, film a umění",
	"technika a domácí spotřebiče",
	"rodina, lidé a vztahy",
	"čas, svátky a tradice",
}

// The lowest band reaches all the way to zero so that a guess the model rates
// as unrelated still lands inside the ranking. If the ranking stopped at 10,
// every such guess would share one rank just past the end of the list.
var bands = []band{
	{70, 90, 260, "silně související slova — typický kontext, části, nadřazené a podřazené pojmy, nejběžnější kolokace"},
	{40, 70, 600, "středně související slova — stejná oblast lidské činnosti nebo prostředí, ale ne bezprostřední souvislost"},
	{0, 40, 1100, "volně související až zcela nesouvisející běžná česká podstatná jména z jiných oblastí"},
}

const systemPrompt = `Jsi generátor obsahu pro českou slovní hru typu Contexto.
Hráč hádá tajné slovo a u každého tipu vidí jeho pořadí podle sémantické blízkosti k tajnému slovu.

Pravidla pro slova:
- pouze podstatná jména v 1. pádě jednotného čísla (výjimka: pomnožná jména),
- spisovná čeština včetně diakritiky, malými písmeny, bez mezer,
- žádná vlastní jména, zkratky, cizí slova nezdomácnělá v češtině, vulgarismy ani hanlivé výrazy,
- žádné duplicity a žádné tvary téhož slova.

Jak běžná slova volit, ti řekne každé zadání zvlášť — drž se ho.

Blízkost hodnoť jako lidskou asociaci (fotbal–branka–hřiště), ne jako podobnost napsaných řetězců.
Odpovídej výhradně platným JSON objektem podle zadaného tvaru, bez komentáře a bez značkování.`

// Difficulty selects how hard the secret word should be. The ranked list
// around it follows the same everyday-vocabulary rules either way.
type Difficulty int

const (
	// Easy is the daily word: concrete, high-frequency, guessable by anyone.
	Easy Difficulty = iota
	// Hard is the weekly word: abstract or lower-frequency, still known to
	// every adult speaker.
	Hard
)

// Generate picks the secret word and builds the full ranking for it.
// recentSecrets are excluded so the same word does not come up twice in a row.
func (c *Client) Generate(ctx context.Context, recentSecrets []string, difficulty Difficulty) (string, []ScoredWord, error) {
	secret, closest, err := c.pickSecret(ctx, recentSecrets, difficulty)
	if err != nil {
		return "", nil, err
	}

	// Avoid list handed to every chunk: the secret plus its closest words, so
	// the wider bands do not simply repeat them.
	avoid := make([]string, 0, len(closest)+1)
	avoid = append(avoid, secret)
	avoid = append(avoid, closest...)

	chunks := planChunks()
	results := make([][]string, len(chunks))
	errs := make([]error, len(chunks))

	var wg sync.WaitGroup
	sem := make(chan struct{}, parallelChunks)
	for i, ch := range chunks {
		wg.Add(1)
		go func(i int, ch chunk) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i], errs[i] = c.generateChunk(ctx, secret, ch, avoid, difficulty)
		}(i, ch)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return "", nil, err
		}
	}

	// Chunks run independently, so the same word can come back from several of
	// them; the first (closest-band) occurrence wins. Deduplicating before the
	// scores are assigned keeps each band evenly filled by the words that
	// actually survive.
	seen := map[string]struct{}{secret: {}}
	keep := func(words []string) []string {
		out := words[:0:0]
		for _, w := range words {
			if _, dup := seen[w]; dup {
				continue
			}
			seen[w] = struct{}{}
			out = append(out, w)
		}
		return out
	}

	ranking := []ScoredWord{{Word: secret, Score: 100}}
	ranking = append(ranking, spread(keep(closest), closestLow, closestHigh)...)

	// planChunks emits chunks in band order, so walking results in order keeps
	// closer bands ahead of farther ones when duplicates are dropped.
	byBand := make([][][]string, len(bands))
	for i, ch := range chunks {
		byBand[ch.bandIndex] = append(byBand[ch.bandIndex], keep(results[i]))
	}
	for i, b := range bands {
		ranking = append(ranking, spread(merge(byBand[i]), b.low, b.high)...)
	}
	return secret, ranking, nil
}

// merge folds the ordered outputs of one band's chunks into a single ordering.
// Chunks of a band differ only by thematic domain, never by closeness, so what
// carries the signal is a word's relative position inside its own chunk: the
// third of thirty is treated as closer than the tenth of twenty.
func merge(parts [][]string) []string {
	type entry struct {
		pos  float64
		part int
		word string
	}
	var all []entry
	for pi, part := range parts {
		for i, w := range part {
			all = append(all, entry{pos: (float64(i) + 0.5) / float64(len(part)), part: pi, word: w})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].pos != all[j].pos {
			return all[i].pos < all[j].pos
		}
		return all[i].part < all[j].part
	})
	out := make([]string, len(all))
	for i, e := range all {
		out[i] = e.word
	}
	return out
}

// spread turns an ordering into scores by handing out evenly spaced slots
// inside (low, high), closest word first. Scores are strictly decreasing, so
// no two words in a band can tie — which is what the rank is read off.
func spread(words []string, low, high float64) []ScoredWord {
	if len(words) == 0 {
		return nil
	}
	step := (high - low) / float64(len(words))
	out := make([]ScoredWord, len(words))
	for i, w := range words {
		out[i] = ScoredWord{Word: w, Score: high - (float64(i)+0.5)*step}
	}
	return out
}

// chunk is one generation call: a slice of a band, optionally pinned to a
// domain so parallel chunks do not converge on the same words.
type chunk struct {
	band      band
	bandIndex int
	count     int
	index, of int
	domain    string
}

func planChunks() []chunk {
	var chunks []chunk
	domainIdx := 0
	for bi, b := range bands {
		parts := (b.count + chunkSize - 1) / chunkSize
		for i := 0; i < parts; i++ {
			count := chunkSize
			if i == parts-1 {
				count = b.count - i*chunkSize
			}
			c := chunk{band: b, bandIndex: bi, count: count, index: i + 1, of: parts}
			// Close bands must stay close to the secret, so only the wider
			// bands get a domain — there the spread is what matters.
			if b.low < 70 {
				c.domain = domains[domainIdx%len(domains)]
				domainIdx++
			}
			chunks = append(chunks, c)
		}
	}
	return chunks
}

const (
	easyVocabRule = `Používej jen slova, která zná každý dospělý bez odborného vzdělání a běžně je použije v hovoru.
Žádné odborné termíny (arachnofobie, hypotenze, kondenzátor), knižní ani archaická slova, regionalismy
a cechovní výrazy. Test: napsal by to běžný člověk do SMS?`

	hardVocabRule = `Slova mají být známá, ale klidně i méně častá — taková, která člověk zná pasivně a v běžném
hovoru je nepoužije každý den (arachnofobie, vertikutátor, mandloň, veranda). Vyhni se jen čistému oborovému
žargonu, který zná pouze daná profese, a archaismům, které dnes nikdo nepoužívá.`

	easySecretRule = `Tajné slovo musí být běžné, konkrétní a obecně známé české podstatné jméno (například: kolo, rybník, polévka, knihovna). Nesmí být abstraktní, odborné ani vzácné.`

	hardSecretRule = `Tajné slovo má být výrazně těžší na uhodnutí než u denní hry. Vyber buď abstraktní pojem
(svědomí, závist, náhoda), věc, na kterou hráč nesáhne hned (průvan, práh, výmol), nebo méně časté slovo,
které lidé znají pasivně (arachnofobie, vertikutátor, mandloň). Hráč po odhalení musí říct „to slovo znám,
jen mě nenapadlo“ — ne „to jsem nikdy neslyšel“. Vyhni se oborovému žargonu a archaismům.`
)

// vocabRule is the wording each prompt gets about how common the words may be.
func (d Difficulty) vocabRule() string {
	if d == Hard {
		return hardVocabRule
	}
	return easyVocabRule
}

func (c *Client) pickSecret(ctx context.Context, recent []string, difficulty Difficulty) (string, []string, error) {
	avoid := "(zatím žádná)"
	if len(recent) > 0 {
		avoid = strings.Join(recent, ", ")
	}
	rule := easySecretRule
	if difficulty == Hard {
		rule = hardSecretRule
	}
	prompt := fmt.Sprintf(`Vyber tajné slovo pro jednu hru a k němu 40 nejbližších slov.

%s

%s
Nepoužívej ani jedno z těchto dříve použitých slov: %s

Jsou to slova, která si člověk s tajným slovem vybaví okamžitě. Tajné slovo samotné do seznamu nedávej.
Seznam seřaď od nejbližšího po nejvzdálenější. Pořadí je jediné, co hodnotíme — žádná čísla nevracej.

Vrať JSON:
{"secret": "…", "words": ["…", "…", …]}`, rule, difficulty.vocabRule(), avoid)

	var out struct {
		Secret string   `json:"secret"`
		Words  []string `json:"words"`
	}
	cfg := call{model: c.model, maxTokens: 16000, thinking: true}
	if err := c.completeJSON(ctx, cfg, prompt, &out); err != nil {
		return "", nil, fmt.Errorf("pick secret: %w", err)
	}
	secret := normalizeWord(out.Secret)
	if secret == "" {
		return "", nil, errors.New("model returned no secret word")
	}
	words := make([]string, 0, len(out.Words))
	for _, w := range out.Words {
		word := normalizeWord(w)
		if word == "" || word == secret {
			continue
		}
		words = append(words, word)
	}
	return secret, words, nil
}

func (c *Client) generateChunk(ctx context.Context, secret string, ch chunk, avoid []string, difficulty Difficulty) ([]string, error) {
	b := ch.band
	focus := ""
	if ch.domain != "" {
		focus = fmt.Sprintf("\nVybírej slova hlavně z oblasti: %s. Ostatní části pásma pokrývají jiné oblasti.\n", ch.domain)
	}
	prompt := fmt.Sprintf(`Tajné slovo je „%s“.%s

Vypiš %d českých slov, jejichž blízkost k tajnému slovu patří do pásma %.0f až %.0f bodů ze 100.
Pásmo obsahuje: %s.

Tohle je část %d z %d tohoto pásma — ostatní části píše někdo jiný, takže se nesnaž pásmo pokrýt celé.
Nepoužívej tato slova: %s

%s

Seznam seřaď od nejbližšího k tajnému slovu po nejvzdálenější. Pořadí v seznamu je jediné, co hodnotíme — žádná čísla nevracej.

Vrať JSON:
{"words": ["…", "…", …]}`, secret, focus, ch.count, b.low, b.high, b.guidance, ch.index, ch.of, strings.Join(avoid, ", "), difficulty.vocabRule())

	var out struct {
		Words []string `json:"words"`
	}
	// Listing and ordering nouns needs no deliberation, and this is the call
	// that runs eleven times per puzzle.
	cfg := call{model: c.bulkModel, maxTokens: 12000, effort: anthropic.OutputConfigEffortLow}
	if err := c.completeJSON(ctx, cfg, prompt, &out); err != nil {
		return nil, fmt.Errorf("band %.0f-%.0f part %d/%d: %w", b.low, b.high, ch.index, ch.of, err)
	}

	words := make([]string, 0, len(out.Words))
	for _, w := range out.Words {
		word := normalizeWord(w)
		if word == "" {
			continue
		}
		words = append(words, word)
	}
	return words, nil
}

// Guess is the model's verdict on a word that is not part of the stored
// ranking: its 0-100 closeness plus the canonical Czech spelling, so a player
// who typed without diacritics still sees the word written properly.
type Guess struct {
	Word  string
	Score float64
}

// Anchor is one already-ranked word shown to the model as a reference point
// when it scores a guess.
type Anchor struct {
	Word  string
	Score float64
}

// ScoreGuess rates a word that is not part of the stored ranking, on the same
// 0-100 scale so the rank falls out of the existing ZSET.
//
// The ranking itself was built by comparison — the model saw a whole band at
// once and put it in order. A guess arrives alone, and rating it in isolation
// is a different, harder task: nothing tells the model that "pekař" already sits
// near the top, so "pekařka" can land hundreds of ranks away from it. Passing a
// handful of ranked words with their scores turns the absolute judgement back
// into a comparison against fixed points.
func (c *Client) ScoreGuess(ctx context.Context, secret, guess string, anchors []Anchor) (Guess, error) {
	scale := `Ohodnoť blízkost tipu k tajnému slovu na stupnici 0 až 100:
- 90-100: okamžitá asociace, prakticky totéž téma,
- 70-90: silná souvislost (typický kontext, část celku, nadřazený či podřazený pojem),
- 40-70: stejná oblast, ale bez bezprostřední souvislosti,
- 10-40: jiná oblast, jen vzdálená souvislost,
- 0-10: bez souvislosti.`

	if len(anchors) > 0 {
		var b strings.Builder
		b.WriteString("Tato slova už v žebříčku jsou a mají tato skóre. Použij je jako měřítko a zasaď tip mezi ně tak, aby výsledek s nimi dával smysl:\n")
		for _, a := range anchors {
			fmt.Fprintf(&b, "- %s = %.1f\n", a.Word, a.Score)
		}
		b.WriteString("\nDrž se tohoto měřítka. Když je tip blízký některému z uvedených slov, musí mít podobné skóre jako ono.\n\n")
		b.WriteString(scale)
		scale = b.String()
	}

	prompt := fmt.Sprintf(`Tajné slovo je „%s“ (skóre 100). Hráč tipuje slovo „%s“.

%s

Hráč mohl slovo napsat bez diakritiky nebo v jiném pádě („zvire“, „psa“). Pokud je zřejmé, které české slovo myslí, ohodnoť ho a do „w“ vrať jeho základní tvar se správnou diakritikou (1. pád jednotného čísla u podstatných jmen).
Nastav known na false jen tehdy, když nejde o české slovo vůbec (nesmysl, cizí slovo).
Skóre vracej na jedno desetinné místo — hráč vidí přesné pořadí, takže na jemných rozdílech záleží.

Vrať JSON:
{"known": true, "w": "zvíře", "s": 37.4}`, secret, guess, scale)

	var out struct {
		Known *bool   `json:"known"`
		W     string  `json:"w"`
		S     float64 `json:"s"`
	}
	// No thinking: the judgement is one comparison, not a chain of reasoning.
	// Effort is left at the model default — with thinking off it barely moves
	// the bill for a reply this small, and this number becomes a rank.
	cfg := call{model: c.guessModel, maxTokens: 1000}
	if err := c.completeJSON(ctx, cfg, prompt, &out); err != nil {
		return Guess{}, fmt.Errorf("score %q: %w", guess, err)
	}
	if out.Known != nil && !*out.Known {
		return Guess{}, ErrUnknownWord
	}
	word := normalizeWord(out.W)
	if word == "" {
		word = guess
	}
	return Guess{Word: word, Score: clamp(out.S, 0, 100)}, nil
}

// completeJSON runs one streamed request and unmarshals the model's text into
// dst. Streaming keeps large generations well clear of HTTP timeouts.
func (c *Client) completeJSON(ctx context.Context, cfg call, prompt string, dst any) error {
	params := anthropic.MessageNewParams{
		Model:     cfg.model,
		MaxTokens: cfg.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}
	if !cfg.thinking {
		disabled := anthropic.NewThinkingConfigDisabledParam()
		params.Thinking = anthropic.ThinkingConfigParamUnion{OfDisabled: &disabled}
	}
	if cfg.effort != "" {
		params.OutputConfig = anthropic.OutputConfigParam{Effort: cfg.effort}
	}

	stream := c.api.Messages.NewStreaming(ctx, params)

	message := anthropic.Message{}
	for stream.Next() {
		if err := message.Accumulate(stream.Current()); err != nil {
			return err
		}
	}
	if err := stream.Err(); err != nil {
		return err
	}
	// One line per API call so the bill is attributable to a job and a model
	// rather than showing up only on the invoice.
	log.Printf("llm %s: in=%d out=%d cache_read=%d",
		cfg.model, message.Usage.InputTokens, message.Usage.OutputTokens, message.Usage.CacheReadInputTokens)

	if message.StopReason == anthropic.StopReasonRefusal {
		return fmt.Errorf("model refused the request: %s", message.StopDetails.Explanation)
	}

	var text strings.Builder
	for _, block := range message.Content {
		if b, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(b.Text)
		}
	}
	payload := extractJSON(text.String())
	if payload == "" {
		return fmt.Errorf("model returned no JSON (stop reason %q)", message.StopReason)
	}
	if err := json.Unmarshal([]byte(payload), dst); err != nil {
		return fmt.Errorf("decode model output: %w", err)
	}
	return nil
}

// extractJSON pulls the outermost JSON object out of the model's text, so a
// stray sentence or a fenced code block does not break decoding.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

// normalizeWord keeps the canonical Czech spelling (diacritics included) but
// drops anything that is not a single lower-case word.
func normalizeWord(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, ".,;:!?\"'`")
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return ""
	}
	for _, r := range s {
		if r == '-' {
			continue
		}
		if !isCzechLetter(r) {
			return ""
		}
	}
	return s
}

func isCzechLetter(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	return strings.ContainsRune("áčďéěíňóřšťúůýž", r)
}

func clamp(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
