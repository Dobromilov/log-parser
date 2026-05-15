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
	"time"
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

func TestLogHandler(t *testing.T) {
	uploadedAt := time.Date(2026, 5, 15, 1, 2, 3, 0, time.UTC)
	repo := &fakeRepository{
		log: &models.Log{
			ID:         "log-1",
			FilePath:   "diagnostic.zip",
			Status:     "parsed",
			NodesCount: 2,
			PortsCount: 3,
			UploadedAt: uploadedAt,
		},
	}

	mux := http.NewServeMux()
	Register(mux, t.TempDir(), repo)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/log/log-1", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var body logResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "log-1" || body.NodesCount != 2 || body.PortsCount != 3 {
		t.Fatalf("unexpected log response: %+v", body)
	}
}

func TestTopologyHandler(t *testing.T) {
	repo := &fakeRepository{
		topology: &models.Topology{
			LogID: "log-1",
			Nodes: []models.TopologyNode{
				{ID: "node-1", ExternalID: "0xhost1", Name: "HOST_1", Type: "host", NumPorts: 1, PortsCount: 1},
			},
			Groups: []models.TopologyGroup{
				{Type: "host", NodeIDs: []string{"node-1"}, Count: 1},
			},
			Links: []models.TopologyLink{},
		},
	}

	mux := http.NewServeMux()
	Register(mux, t.TempDir(), repo)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/topology/log-1", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var body topologyResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.LogID != "log-1" || len(body.Nodes) != 1 || len(body.Groups) != 1 {
		t.Fatalf("unexpected topology response: %+v", body)
	}
}

func TestNodeHandler(t *testing.T) {
	repo := &fakeRepository{
		node: &models.Node{
			ID:         "node-1",
			LogID:      "log-1",
			ExternalID: "0xhost1",
			Name:       "HOST_1",
			Type:       "host",
			NumPorts:   1,
			NodeGUID:   "0xhost1",
			Raw:        map[string]string{"NodeGUID": "0xhost1"},
		},
	}

	mux := http.NewServeMux()
	Register(mux, t.TempDir(), repo)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/node/node-1", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var body nodeResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.ID != "node-1" || body.Name != "HOST_1" {
		t.Fatalf("unexpected node response: %+v", body)
	}
}

func TestPortsHandler(t *testing.T) {
	repo := &fakeRepository{
		ports: []models.Port{
			{
				ID:              "port-1",
				LogID:           "log-1",
				NodeID:          "node-1",
				NodeGUID:        "0xhost1",
				PortGUID:        "0xhost1",
				PortNum:         1,
				LID:             "1",
				PortPhyState:    5,
				PortStateCode:   4,
				PortState:       "active",
				LinkWidthActive: "2",
				LinkSpeedActive: "2048",
			},
		},
	}

	mux := http.NewServeMux()
	Register(mux, t.TempDir(), repo)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/port/node-1", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}

	var body portsResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.NodeID != "node-1" || len(body.Ports) != 1 || body.Ports[0].PortState != "active" {
		t.Fatalf("unexpected ports response: %+v", body)
	}
}

func TestReadHandlersRequireDatabase(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, t.TempDir())

	request := httptest.NewRequest(http.MethodGet, "/api/v1/log/log-1", nil)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", response.Code)
	}
}

type fakeRepository struct {
	filePath string
	result   *models.ParseResult
	log      *models.Log
	topology *models.Topology
	node     *models.Node
	info     *models.NodeInfo
	ports    []models.Port
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

func (f *fakeRepository) GetLog(ctx context.Context, logID string) (*models.Log, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.log == nil {
		return nil, models.ErrNotFound
	}

	return f.log, nil
}

func (f *fakeRepository) GetTopology(ctx context.Context, logID string) (*models.Topology, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.topology == nil {
		return nil, models.ErrNotFound
	}

	return f.topology, nil
}

func (f *fakeRepository) GetNode(ctx context.Context, nodeID string) (*models.Node, *models.NodeInfo, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.node == nil {
		return nil, nil, models.ErrNotFound
	}

	return f.node, f.info, nil
}

func (f *fakeRepository) GetPortsByNodeID(ctx context.Context, nodeID string) ([]models.Port, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.ports == nil {
		return nil, models.ErrNotFound
	}

	return f.ports, nil
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
