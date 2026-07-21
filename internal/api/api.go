package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vojtechpastyrik/slovolov/internal/game"
)

type Server struct {
	game *game.Service
}

func NewServer(g *game.Service) *Server {
	return &Server{game: g}
}

func (s *Server) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/game", s.handleNewGame)
	mux.HandleFunc("POST /api/game/{gameID}/guess", s.handleGuess)
	mux.HandleFunc("POST /api/game/{gameID}/hint", s.handleHint)
	mux.HandleFunc("POST /api/game/{gameID}/reveal", s.handleReveal)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

type newGameResponse struct {
	GameID     string `json:"gameId"`
	WordLength int    `json:"wordLength"`
	HintsMax   int    `json:"hintsMax"`
}

func (s *Server) handleNewGame(w http.ResponseWriter, r *http.Request) {
	res, err := s.game.NewGame(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, newGameResponse{
		GameID:     res.GameID,
		WordLength: res.WordLength,
		HintsMax:   game.MaxHints,
	})
}

type guessRequest struct {
	Word string `json:"word"`
}

func (s *Server) handleGuess(w http.ResponseWriter, r *http.Request) {
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

	result, err := s.game.Guess(r.Context(), gameID, req.Word)
	if errors.Is(err, game.ErrGameNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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
	case errors.Is(err, game.ErrHintLimit):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, game.ErrNoHintAvailable):
		writeError(w, http.StatusServiceUnavailable, err)
	case err != nil:
		writeError(w, http.StatusInternalServerError, err)
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
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, revealResponse{Word: secret})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
