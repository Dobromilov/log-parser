package models

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Log struct {
	ID           string
	FilePath     string
	Status       string
	ErrorMessage *string
	NodesCount   int
	PortsCount   int
	UploadedAt   time.Time
	ParsedAt     *time.Time
}

type Node struct {
	ID         string
	LogID      string
	ExternalID string
	Name       string
	Type       string
	NumPorts   int
	NodeGUID   string
	Raw        map[string]string
}

type Port struct {
	ID              string
	LogID           string
	NodeID          string
	NodeGUID        string
	PortGUID        string
	PortNum         int
	LID             string
	PortPhyState    int
	PortStateCode   int
	PortState       string
	LinkWidthActive string
	LinkSpeedActive string
	Raw             map[string]string
}

type NodeInfo struct {
	ID           string
	LogID        string
	NodeID       string
	NodeGUID     string
	SerialNumber string
	PartNumber   string
	Revision     string
	ProductName  string
	Raw          map[string]string
}

type ParseResult struct {
	Nodes     []Node
	Ports     []Port
	NodeInfos []NodeInfo
}

type TopologyNode struct {
	ID         string
	ExternalID string
	Name       string
	Type       string
	NumPorts   int
	PortsCount int
}

type TopologyGroup struct {
	Type    string
	NodeIDs []string
	Count   int
}

type Topology struct {
	LogID  string
	Nodes  []TopologyNode
	Groups []TopologyGroup
	Links  []TopologyLink
}

type TopologyLink struct {
	SourceNodeID string
	SourcePortID string
	TargetNodeID string
	TargetPortID string
	Confidence   string
}
