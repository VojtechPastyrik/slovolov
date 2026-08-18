package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/vojtechpastyrik/slovolov/internal/game"
)

type Server struct {
	game *game.Service
}

func NewServer(g *game.Service) *Server {
	return &Server{game: g}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/game", s.handleCurrent(game.ModeDay))
	mux.HandleFunc("GET /api/game/today", s.handleCurrent(game.ModeDay))
	mux.HandleFunc("GET /api/game/week", s.handleCurrent(game.ModeWeek))
	mux.HandleFunc("POST /api/game/{gameID}/guess", s.withSession(s.handleGuess))
	mux.HandleFunc("GET /api/game/{gameID}/stats", s.handleStats)
	mux.HandleFunc("GET /api/game/today/previous", s.handlePrevious(game.ModeDay))
	mux.HandleFunc("GET /api/game/week/previous", s.handlePrevious(game.ModeWeek))
	mux.HandleFunc("POST /api/game/{gameID}/hint", s.handleHint)
	mux.HandleFunc("POST /api/game/{gameID}/reveal", s.handleReveal)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

type puzzleResponse struct {
	Mode       string `json:"mode"`
	GameID     string `json:"gameId"`
	WordLength int    `json:"wordLength"`
	HintsMax   int    `json:"hintsMax"`
	ResetsAt   string `json:"resetsAt"`
}

// handleCurrent returns the running puzzle for a mode. One word per period, so
// the call is idempotent — every player gets the same gameId until it rolls
// over.
func (s *Server) handleCurrent(mode game.Mode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := s.game.Current(r.Context(), mode)
		if errors.Is(err, game.ErrPuzzlePending) {
			w.Header().Set("Retry-After", "15")
			writeError(w, http.StatusServiceUnavailable, errors.New("slovo se připravuje"))
			return
		}
		if err != nil {
			writeServerError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, puzzleResponse{
			Mode:       string(res.Mode),
			GameID:     res.ID,
			WordLength: res.WordLength,
			HintsMax:   game.MaxHints,
			ResetsAt:   res.ResetsAt.Format(time.RFC3339),
		})
	}
}

// sessionCookie identifies a player across requests so the server can count
// their guesses itself. It carries no personal data and is not shared with
// anyone — it exists because a guess tally reported by the client is a number
// the client can simply make up.
const sessionCookie = "slovolov_sid"

// withSession hands the handler a stable per-browser id, minting one on the
// first guess. Nothing else on the site sets a cookie, so a visitor who only
// reads the page never gets one.
func (s *Server) withSession(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := ""
		if c, err := r.Cookie(sessionCookie); err == nil && validSession(c.Value) {
			session = c.Value
		}
		if session == "" {
			raw := make([]byte, 16)
			if _, err := rand.Read(raw); err != nil {
				writeServerError(w, r, err)
				return
			}
			session = base64.RawURLEncoding.EncodeToString(raw)
			http.SetCookie(w, &http.Cookie{
				Name:     sessionCookie,
				Value:    session,
				Path:     "/",
				MaxAge:   int((400 * 24 * time.Hour).Seconds()),
				HttpOnly: true,
				Secure:   isHTTPS(r),
				SameSite: http.SameSiteLaxMode,
			})
		}
		next(w, r, session)
	}
}

// validSession rejects anything that is not a value this server minted, so a
// hand-crafted cookie cannot steer the key a session writes to.
func validSession(v string) bool {
	if len(v) != base64.RawURLEncoding.EncodedLen(16) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

type guessRequest struct {
	Word string `json:"word"`
}

func (s *Server) handleGuess(w http.ResponseWriter, r *http.Request, session string) {
	gameID := r.PathValue("gameID")
	if gameID == "" {
		writeError(w, http.StatusBadRequest, errors.New("gameID is required"))
		return
	}

	var req guessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Word == "" {
		writeError(w, http.StatusBadRequest, errors.New("word is required"))
		return
	}

	result, err := s.game.Guess(r.Context(), gameID, session, req.Word)
	switch {
	case errors.Is(err, game.ErrGameNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, game.ErrUnknownWord):
		writeError(w, http.StatusUnprocessableEntity, errors.New("toto slovo neznám"))
	case err != nil:
		writeServerError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

type hintRequest struct {
	BestRank int64    `json:"bestRank"`
	Exclude  []string `json:"exclude"`
}

func (s *Server) handleHint(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	if gameID == "" {
		writeError(w, http.StatusBadRequest, errors.New("gameID is required"))
		return
	}
	var req hintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.game.Hint(r.Context(), gameID, req.BestRank, req.Exclude)
	switch {
	case errors.Is(err, game.ErrGameNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, game.ErrNoHintAvailable):
		writeError(w, http.StatusServiceUnavailable, err)
	case err != nil:
		writeServerError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, result)
	}
}

type revealResponse struct {
	Word string `json:"word"`
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	if gameID == "" {
		writeError(w, http.StatusBadRequest, errors.New("gameID is required"))
		return
	}
	secret, err := s.game.Reveal(r.Context(), gameID)
	if errors.Is(err, game.ErrGameNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, revealResponse{Word: secret})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	gameID := r.PathValue("gameID")
	if gameID == "" {
		writeError(w, http.StatusBadRequest, errors.New("gameID is required"))
		return
	}
	stats, err := s.game.Stats(r.Context(), gameID)
	if err != nil {
		writeServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// handlePrevious reveals the word of the mode's last finished puzzle. Only the
// finished one — the running puzzle is never reachable through here.
func (s *Server) handlePrevious(mode game.Mode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := s.game.Previous(r.Context(), mode)
		if errors.Is(err, game.ErrGameNotFound) {
			writeError(w, http.StatusNotFound, errors.New("žádná předchozí hra"))
			return
		}
		if err != nil {
			writeServerError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

// writeServerError keeps upstream detail (API URLs, provider messages) in the
// log and hands the client a generic message.
func writeServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "něco se pokazilo, zkus to za chvíli"})
}
