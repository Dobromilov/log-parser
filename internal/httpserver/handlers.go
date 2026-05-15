package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log-parser/internal/models"
	"log-parser/internal/parser"
	"log/slog"
	"net/http"
	"time"
)

type Repository interface {
	SaveParseResult(ctx context.Context, filePath string, result *models.ParseResult) (*models.Log, error)
	GetLog(ctx context.Context, logID string) (*models.Log, error)
	GetTopology(ctx context.Context, logID string) (*models.Topology, error)
	GetNode(ctx context.Context, nodeID string) (*models.Node, *models.NodeInfo, error)
	GetPortsByNodeID(ctx context.Context, nodeID string) ([]models.Port, error)
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

type logResponse struct {
	ID           string     `json:"id"`
	FilePath     string     `json:"file_path"`
	Status       string     `json:"status"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	NodesCount   int        `json:"nodes_count"`
	PortsCount   int        `json:"ports_count"`
	UploadedAt   time.Time  `json:"uploaded_at"`
	ParsedAt     *time.Time `json:"parsed_at,omitempty"`
}

type topologyResponse struct {
	LogID  string                  `json:"log_id"`
	Nodes  []topologyNodeResponse  `json:"nodes"`
	Groups []topologyGroupResponse `json:"groups"`
	Links  []topologyLinkResponse  `json:"links"`
}

type topologyNodeResponse struct {
	ID         string `json:"id"`
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	NumPorts   int    `json:"num_ports"`
	PortsCount int    `json:"ports_count"`
}

type topologyGroupResponse struct {
	Type    string   `json:"type"`
	NodeIDs []string `json:"node_ids"`
	Count   int      `json:"count"`
}

type topologyLinkResponse struct {
	SourceNodeID string `json:"source_node_id"`
	SourcePortID string `json:"source_port_id"`
	TargetNodeID string `json:"target_node_id"`
	TargetPortID string `json:"target_port_id"`
	Confidence   string `json:"confidence"`
}

type nodeResponse struct {
	ID         string            `json:"id"`
	LogID      string            `json:"log_id"`
	ExternalID string            `json:"external_id"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	NumPorts   int               `json:"num_ports"`
	NodeGUID   string            `json:"node_guid"`
	Info       *nodeInfoResponse `json:"info,omitempty"`
	Raw        map[string]string `json:"raw,omitempty"`
}

type nodeInfoResponse struct {
	ID           string            `json:"id"`
	LogID        string            `json:"log_id"`
	NodeID       string            `json:"node_id"`
	NodeGUID     string            `json:"node_guid"`
	SerialNumber string            `json:"serial_number"`
	PartNumber   string            `json:"part_number"`
	Revision     string            `json:"revision"`
	ProductName  string            `json:"product_name"`
	Raw          map[string]string `json:"raw,omitempty"`
}

type portsResponse struct {
	NodeID string         `json:"node_id"`
	Ports  []portResponse `json:"ports"`
}

type portResponse struct {
	ID              string            `json:"id"`
	LogID           string            `json:"log_id"`
	NodeID          string            `json:"node_id"`
	NodeGUID        string            `json:"node_guid"`
	PortGUID        string            `json:"port_guid"`
	PortNum         int               `json:"port_num"`
	LID             string            `json:"lid"`
	PortPhyState    int               `json:"port_phy_state"`
	PortStateCode   int               `json:"port_state_code"`
	PortState       string            `json:"port_state"`
	LinkWidthActive string            `json:"link_width_active"`
	LinkSpeedActive string            `json:"link_speed_active"`
	Raw             map[string]string `json:"raw,omitempty"`
}

func Register(mux *http.ServeMux, dataDir string, repositories ...Repository) {
	var repo Repository
	if len(repositories) > 0 {
		repo = repositories[0]
	}

	server := &Server{dataDir: dataDir, repo: repo}

	mux.HandleFunc("/health", Health)
	mux.HandleFunc("/api/v1/parse/", server.Parse)
	mux.HandleFunc("/api/v1/log/", server.Log)
	mux.HandleFunc("/api/v1/topology/", server.Topology)
	mux.HandleFunc("/api/v1/node/", server.Node)
	mux.HandleFunc("/api/v1/port/", server.Ports)
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

func (s *Server) Log(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !s.requireRepository(w) {
		return
	}

	logID, ok := pathID(w, r, "/api/v1/log/")
	if !ok {
		return
	}

	logRecord, err := s.repo.GetLog(r.Context(), logID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toLogResponse(logRecord))
}

func (s *Server) Topology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !s.requireRepository(w) {
		return
	}

	logID, ok := pathID(w, r, "/api/v1/topology/")
	if !ok {
		return
	}

	topology, err := s.repo.GetTopology(r.Context(), logID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toTopologyResponse(topology))
}

func (s *Server) Node(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !s.requireRepository(w) {
		return
	}

	nodeID, ok := pathID(w, r, "/api/v1/node/")
	if !ok {
		return
	}

	node, info, err := s.repo.GetNode(r.Context(), nodeID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toNodeResponse(node, info))
}

func (s *Server) Ports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !s.requireRepository(w) {
		return
	}

	nodeID, ok := pathID(w, r, "/api/v1/port/")
	if !ok {
		return
	}

	ports, err := s.repo.GetPortsByNodeID(r.Context(), nodeID)
	if err != nil {
		writeRepositoryError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toPortsResponse(nodeID, ports))
}

func (s *Server) requireRepository(w http.ResponseWriter) bool {
	if s.repo == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database is not configured"})
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeRepositoryError(w http.ResponseWriter, err error) {
	if errors.Is(err, models.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}

	slog.Error("repository request failed", "error", err)
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
}

func pathID(w http.ResponseWriter, r *http.Request, prefix string) (string, bool) {
	id := r.URL.Path[len(prefix):]
	if id == "" || id == "/" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "id is required"})
		return "", false
	}

	return id, true
}

func toLogResponse(logRecord *models.Log) logResponse {
	return logResponse{
		ID:           logRecord.ID,
		FilePath:     logRecord.FilePath,
		Status:       logRecord.Status,
		ErrorMessage: logRecord.ErrorMessage,
		NodesCount:   logRecord.NodesCount,
		PortsCount:   logRecord.PortsCount,
		UploadedAt:   logRecord.UploadedAt,
		ParsedAt:     logRecord.ParsedAt,
	}
}

func toTopologyResponse(topology *models.Topology) topologyResponse {
	response := topologyResponse{
		LogID:  topology.LogID,
		Nodes:  make([]topologyNodeResponse, 0, len(topology.Nodes)),
		Groups: make([]topologyGroupResponse, 0, len(topology.Groups)),
		Links:  make([]topologyLinkResponse, 0, len(topology.Links)),
	}

	for _, node := range topology.Nodes {
		response.Nodes = append(response.Nodes, topologyNodeResponse{
			ID:         node.ID,
			ExternalID: node.ExternalID,
			Name:       node.Name,
			Type:       node.Type,
			NumPorts:   node.NumPorts,
			PortsCount: node.PortsCount,
		})
	}

	for _, group := range topology.Groups {
		response.Groups = append(response.Groups, topologyGroupResponse{
			Type:    group.Type,
			NodeIDs: group.NodeIDs,
			Count:   group.Count,
		})
	}

	for _, link := range topology.Links {
		response.Links = append(response.Links, topologyLinkResponse{
			SourceNodeID: link.SourceNodeID,
			SourcePortID: link.SourcePortID,
			TargetNodeID: link.TargetNodeID,
			TargetPortID: link.TargetPortID,
			Confidence:   link.Confidence,
		})
	}

	return response
}

func toNodeResponse(node *models.Node, info *models.NodeInfo) nodeResponse {
	response := nodeResponse{
		ID:         node.ID,
		LogID:      node.LogID,
		ExternalID: node.ExternalID,
		Name:       node.Name,
		Type:       node.Type,
		NumPorts:   node.NumPorts,
		NodeGUID:   node.NodeGUID,
		Raw:        node.Raw,
	}

	if info != nil {
		response.Info = &nodeInfoResponse{
			ID:           info.ID,
			LogID:        info.LogID,
			NodeID:       info.NodeID,
			NodeGUID:     info.NodeGUID,
			SerialNumber: info.SerialNumber,
			PartNumber:   info.PartNumber,
			Revision:     info.Revision,
			ProductName:  info.ProductName,
			Raw:          info.Raw,
		}
	}

	return response
}

func toPortsResponse(nodeID string, ports []models.Port) portsResponse {
	response := portsResponse{
		NodeID: nodeID,
		Ports:  make([]portResponse, 0, len(ports)),
	}

	for _, port := range ports {
		response.Ports = append(response.Ports, portResponse{
			ID:              port.ID,
			LogID:           port.LogID,
			NodeID:          port.NodeID,
			NodeGUID:        port.NodeGUID,
			PortGUID:        port.PortGUID,
			PortNum:         port.PortNum,
			LID:             port.LID,
			PortPhyState:    port.PortPhyState,
			PortStateCode:   port.PortStateCode,
			PortState:       port.PortState,
			LinkWidthActive: port.LinkWidthActive,
			LinkSpeedActive: port.LinkSpeedActive,
			Raw:             port.Raw,
		})
	}

	return response
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
