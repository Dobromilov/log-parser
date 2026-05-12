package parser

import (
	"strings"
	"testing"
)

func TestParseDBCSV(t *testing.T) {
	input := `START_NODES
NodeDesc,NumPorts,NodeType,ClassVersion,BaseVersion,SystemImageGUID,NodeGUID,PortGUID
"HOST_1",1,1,1,1,0xhost1,0xhost1,0xhost1
"SWITCH_1",65,2,1,1,0xswitch1,0xswitch1,0xswitch1
END_NODES

START_PORTS
NodeGuid,PortGuid,PortNum,MKey,GIDPrfx,MSMLID,LID,CapMsk,M_KeyLeasePeriod,DiagCode,LinkWidthActv,LinkWidthSup,LinkWidthEn,LocalPortNum,LinkSpeedEn,LinkSpeedActv,LMC,MKeyProtBits,LinkDownDefState,PortPhyState,PortState
0xhost1,0xhost1,1,0xffffffffff,0xffffffffff,1,1,2807162954,60,0,2,19,19,1,3841,2048,0,0,2,5,4
0xswitch1,0xswitch1,0,0xffffffffff,0xffffffffff,1,22,3847280712,60,0,0,0,0,1,0,0,0,0,0,0,0
END_PORTS

START_SYSTEM_GENERAL_INFORMATION
NodeGuid,SerialNumber,PartNumber,Revision,ProductName
0xswitch1,SOS123,MMM-MAV,AA,"Gorilla"
END_SYSTEM_GENERAL_INFORMATION
`

	result, err := ParseDBCSV(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseDBCSV returned error: %v", err)
	}

	if len(result.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result.Nodes))
	}
	if len(result.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(result.Ports))
	}
	if len(result.NodeInfos) != 1 {
		t.Fatalf("expected 1 node info, got %d", len(result.NodeInfos))
	}
	if result.Nodes[0].Type != "host" {
		t.Fatalf("expected first node type host, got %s", result.Nodes[0].Type)
	}
	if result.Ports[0].PortState != "active" {
		t.Fatalf("expected first port state active, got %s", result.Ports[0].PortState)
	}
	if result.Ports[0].PortPhyState != 5 {
		t.Fatalf("expected first port physical state 5, got %d", result.Ports[0].PortPhyState)
	}
	if result.NodeInfos[0].ProductName != "Gorilla" {
		t.Fatalf("expected product name Gorilla, got %s", result.NodeInfos[0].ProductName)
	}
}

func TestParseDBCSVRejectsUnknownPortNode(t *testing.T) {
	input := `START_NODES
NodeDesc,NumPorts,NodeType,ClassVersion,BaseVersion,SystemImageGUID,NodeGUID,PortGUID
"HOST_1",1,1,1,1,0xhost1,0xhost1,0xhost1
END_NODES

START_PORTS
NodeGuid,PortGuid,PortNum,LID,LinkWidthActv,LinkSpeedActv,PortPhyState,PortState
0xmissing,0xmissing,1,1,2,2048,5,4
END_PORTS
`

	_, err := ParseDBCSV(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error for unknown port node")
	}
}
