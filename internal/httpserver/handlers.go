package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log-parser/internal/models"
	"log-parser/internal/parser"
	"log/slog"
	"net/http"
	"time"
)

type Repository interface {
	SaveParseResult(ctx context.Context, filePath string, result *models.ParseResult) (*models.Log, error)
}

type Server struct {
	dataDir string
	repo    Repository
}

type parseRequest struct {
	Path string `json:"path"`
}

type parseResponse struct {
	LogID          string `json:"log_id"`
	Status         string `json:"status"`
	NodesCount     int    `json:"nodes_count"`
	PortsCount     int    `json:"ports_count"`
	NodeInfosCount int    `json:"node_infos_count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func Register(mux *http.ServeMux, dataDir string, repositories ...Repository) {
	var repo Repository
	if len(repositories) > 0 {
		repo = repositories[0]
	}

	server := &Server{dataDir: dataDir, repo: repo}

	mux.HandleFunc("/health", Health)
	mux.HandleFunc("/api/v1/parse/", server.Parse)
}

func Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) Parse(w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now()

	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var request parseRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json body"})
		return
	}
	if request.Path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "path is required"})
		return
	}

	result, err := parser.ParseArchive(s.dataDir, request.Path)
	if err != nil {
		slog.Error("parse archive failed",
			"path", request.Path,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"error", err,
		)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	logID := ""
	if s.repo != nil {
		logRecord, err := s.repo.SaveParseResult(r.Context(), request.Path, result)
		if err != nil {
			slog.Error("save parsed log failed",
				"path", request.Path,
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"error", err,
			)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "save parsed log"})
			return
		}
		logID = logRecord.ID
	} else {
		var err error
		logID, err = newLogID()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "generate log id"})
			return
		}
	}

	slog.Info("archive parsed",
		"log_id", logID,
		"path", request.Path,
		"nodes_count", len(result.Nodes),
		"ports_count", len(result.Ports),
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)

	writeJSON(w, http.StatusOK, parseResponse{
		LogID:          logID,
		Status:         "parsed",
		NodesCount:     len(result.Nodes),
		PortsCount:     len(result.Ports),
		NodeInfosCount: len(result.NodeInfos),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newLogID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
