package parser

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const archiveTestDBCSV = `START_NODES
NodeDesc,NumPorts,NodeType,ClassVersion,BaseVersion,SystemImageGUID,NodeGUID,PortGUID
"HOST_1",1,1,1,1,0xhost1,0xhost1,0xhost1
"SWITCH_1",65,2,1,1,0xswitch1,0xswitch1,0xswitch1
END_NODES

START_PORTS
NodeGuid,PortGuid,PortNum,LID,LinkWidthActv,LinkSpeedActv,PortPhyState,PortState
0xhost1,0xhost1,1,1,2,2048,5,4
0xswitch1,0xswitch1,0,22,0,0,0,0
END_PORTS

START_SYSTEM_GENERAL_INFORMATION
NodeGuid,SerialNumber,PartNumber,Revision,ProductName
0xswitch1,SOS123,MMM-MAV,AA,"Gorilla"
END_SYSTEM_GENERAL_INFORMATION
`

func TestParseArchive(t *testing.T) {
	dataDir := t.TempDir()
	archivePath := filepath.Join(dataDir, "diagnostic.zip")

	writeTestArchive(t, archivePath, map[string]string{
		"nested/ibdiagnet2.db_csv": archiveTestDBCSV,
	})

	result, err := ParseArchive(dataDir, "diagnostic.zip")
	if err != nil {
		t.Fatalf("ParseArchive returned error: %v", err)
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
}

func TestParseArchiveRejectsPathTraversal(t *testing.T) {
	_, err := ParseArchive(t.TempDir(), "../diagnostic.zip")
	if err == nil {
		t.Fatal("expected path traversal error")
	}
	if !strings.Contains(err.Error(), "escapes data dir") {
		t.Fatalf("expected escapes data dir error, got %v", err)
	}
}

func TestParseArchiveRejectsMissingDBCSV(t *testing.T) {
	dataDir := t.TempDir()
	archivePath := filepath.Join(dataDir, "diagnostic.zip")

	writeTestArchive(t, archivePath, map[string]string{
		"README.txt": "empty",
	})

	_, err := ParseArchive(dataDir, "diagnostic.zip")
	if err == nil {
		t.Fatal("expected missing db_csv error")
	}
	if !strings.Contains(err.Error(), dbCSVFileName) {
		t.Fatalf("expected db_csv error, got %v", err)
	}
}

func writeTestArchive(t *testing.T, archivePath string, files map[string]string) {
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
