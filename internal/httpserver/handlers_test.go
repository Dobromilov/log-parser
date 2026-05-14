package httpserver

import (
	"archive/zip"
	"context"
	"encoding/json"
	"log-parser/internal/models"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const handlerTestDBCSV = `START_NODES
NodeDesc,NumPorts,NodeType,ClassVersion,BaseVersion,SystemImageGUID,NodeGUID,PortGUID
"HOST_1",1,1,1,1,0xhost1,0xhost1,0xhost1
END_NODES

START_PORTS
NodeGuid,PortGuid,PortNum,LID,LinkWidthActv,LinkSpeedActv,PortPhyState,PortState
0xhost1,0xhost1,1,1,2,2048,5,4
END_PORTS
`

func TestParseHandler(t *testing.T) {
	dataDir := t.TempDir()
	writeHandlerTestArchive(t, filepath.Join(dataDir, "diagnostic.zip"), map[string]string{
		"ibdiagnet2.db_csv": handlerTestDBCSV,
	})

	mux := http.NewServeMux()
	Register(mux, dataDir)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/parse/", strings.NewReader(`{"path":"diagnostic.zip"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var body parseResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.LogID == "" {
		t.Fatal("expected log id")
	}
	if body.Status != "parsed" {
		t.Fatalf("expected status parsed, got %s", body.Status)
	}
	if body.NodesCount != 1 {
		t.Fatalf("expected 1 node, got %d", body.NodesCount)
	}
	if body.PortsCount != 1 {
		t.Fatalf("expected 1 port, got %d", body.PortsCount)
	}
}

func TestParseHandlerSavesResult(t *testing.T) {
	dataDir := t.TempDir()
	writeHandlerTestArchive(t, filepath.Join(dataDir, "diagnostic.zip"), map[string]string{
		"ibdiagnet2.db_csv": handlerTestDBCSV,
	})

	repo := &fakeRepository{
		log: &models.Log{
			ID:     "11111111-1111-4111-8111-111111111111",
			Status: "parsed",
		},
	}

	mux := http.NewServeMux()
	Register(mux, dataDir, repo)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/parse/", strings.NewReader(`{"path":"diagnostic.zip"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if repo.filePath != "diagnostic.zip" {
		t.Fatalf("expected saved path diagnostic.zip, got %s", repo.filePath)
	}
	if repo.result == nil {
		t.Fatal("expected saved parse result")
	}

	var body parseResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.LogID != repo.log.ID {
		t.Fatalf("expected log id from repository, got %s", body.LogID)
	}
}

func TestParseHandlerRejectsPathTraversal(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, t.TempDir())

	request := httptest.NewRequest(http.MethodPost, "/api/v1/parse/", strings.NewReader(`{"path":"../diagnostic.zip"}`))
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
}

type fakeRepository struct {
	filePath string
	result   *models.ParseResult
	log      *models.Log
	err      error
}

func (f *fakeRepository) SaveParseResult(ctx context.Context, filePath string, result *models.ParseResult) (*models.Log, error) {
	f.filePath = filePath
	f.result = result

	if f.err != nil {
		return nil, f.err
	}

	return f.log, nil
}

func writeHandlerTestArchive(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create archive entry: %v", err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write archive entry: %v", err)
		}
	}
}
