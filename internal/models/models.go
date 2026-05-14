package models

import "time"

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
	ExternalID string
	Name       string
	Type       string
	NumPorts   int
	NodeGUID   string
	Raw        map[string]string
}

type Port struct {
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
