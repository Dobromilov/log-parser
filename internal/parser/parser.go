package parser

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log-parser/internal/models"
	"strconv"
	"strings"
)

func ParseDBCSV(r io.Reader) (*models.ParseResult, error) {
	sections, err := readSections(r)
	if err != nil {
		return nil, err
	}

	nodeRows, err := parseCSVSection(sections, "NODES")
	if err != nil {
		return nil, err
	}

	portRows, err := parseCSVSection(sections, "PORTS")
	if err != nil {
		return nil, err
	}

	infoRows, err := parseOptionalCSVSection(sections, "SYSTEM_GENERAL_INFORMATION")
	if err != nil {
		return nil, err
	}

	nodes, err := parseNodes(nodeRows)
	if err != nil {
		return nil, err
	}

	ports, err := parsePorts(portRows)
	if err != nil {
		return nil, err
	}

	nodeInfos, err := parseNodeInfos(infoRows)
	if err != nil {
		return nil, err
	}

	result := &models.ParseResult{
		Nodes:     nodes,
		Ports:     ports,
		NodeInfos: nodeInfos,
	}

	if err := validateResult(result); err != nil {
		return nil, err
	}

	return result, nil
}

func readSections(r io.Reader) (map[string]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	sections := make(map[string]string)
	var currentName string
	var current strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "START_") {
			if currentName != "" {
				return nil, fmt.Errorf("section %s was not closed", currentName)
			}

			currentName = strings.TrimPrefix(line, "START_")
			current.Reset()
			continue
		}

		if strings.HasPrefix(line, "END_") {
			endName := strings.TrimPrefix(line, "END_")
			if currentName == "" {
				return nil, fmt.Errorf("unexpected end section %s", endName)
			}
			if endName != currentName {
				return nil, fmt.Errorf("section %s closed as %s", currentName, endName)
			}

			sections[currentName] = current.String()
			currentName = ""
			current.Reset()
			continue
		}

		if currentName != "" {
			current.WriteString(scanner.Text())
			current.WriteByte('\n')
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if currentName != "" {
		return nil, fmt.Errorf("section %s was not closed", currentName)
	}

	return sections, nil
}

func parseCSVSection(sections map[string]string, name string) ([]map[string]string, error) {
	content, ok := sections[name]
	if !ok || strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("required section %s not found", name)
	}

	return parseCSVRows(name, content)
}

func parseOptionalCSVSection(sections map[string]string, name string) ([]map[string]string, error) {
	content, ok := sections[name]
	if !ok || strings.TrimSpace(content) == "" {
		return nil, nil
	}

	return parseCSVRows(name, content)
}

func parseCSVRows(sectionName, content string) ([]map[string]string, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse section %s: %w", sectionName, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("section %s is empty", sectionName)
	}

	headers := records[0]
	rows := make([]map[string]string, 0, len(records)-1)

	for i, record := range records[1:] {
		if len(record) != len(headers) {
			return nil, fmt.Errorf("section %s row %d: expected %d fields, got %d", sectionName, i+2, len(headers), len(record))
		}

		row := make(map[string]string, len(headers))
		for j, header := range headers {
			row[header] = record[j]
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func parseNodes(rows []map[string]string) ([]models.Node, error) {
	nodes := make([]models.Node, 0, len(rows))

	for i, row := range rows {
		nodeGUID := strings.TrimSpace(row["NodeGUID"])
		if nodeGUID == "" {
			return nil, fmt.Errorf("nodes row %d: NodeGUID is required", i+1)
		}

		numPorts, err := parseInt(row["NumPorts"])
		if err != nil {
			return nil, fmt.Errorf("nodes row %d: invalid NumPorts: %w", i+1, err)
		}

		nodes = append(nodes, models.Node{
			ExternalID: nodeGUID,
			Name:       row["NodeDesc"],
			Type:       normalizeNodeType(row["NodeType"]),
			NumPorts:   numPorts,
			NodeGUID:   nodeGUID,
			Raw:        cloneRow(row),
		})
	}

	return nodes, nil
}

func parsePorts(rows []map[string]string) ([]models.Port, error) {
	ports := make([]models.Port, 0, len(rows))

	for i, row := range rows {
		nodeGUID := strings.TrimSpace(row["NodeGuid"])
		if nodeGUID == "" {
			return nil, fmt.Errorf("ports row %d: NodeGuid is required", i+1)
		}

		portNum, err := parseInt(row["PortNum"])
		if err != nil {
			return nil, fmt.Errorf("ports row %d: invalid PortNum: %w", i+1, err)
		}

		portStateCode, err := parseInt(row["PortState"])
		if err != nil {
			return nil, fmt.Errorf("ports row %d: invalid PortState: %w", i+1, err)
		}

		portPhyState, err := parseInt(row["PortPhyState"])
		if err != nil {
			return nil, fmt.Errorf("ports row %d: invalid PortPhyState: %w", i+1, err)
		}

		ports = append(ports, models.Port{
			NodeGUID:        nodeGUID,
			PortGUID:        row["PortGuid"],
			PortNum:         portNum,
			LID:             row["LID"],
			PortPhyState:    portPhyState,
			PortStateCode:   portStateCode,
			PortState:       normalizePortState(portStateCode),
			LinkWidthActive: row["LinkWidthActv"],
			LinkSpeedActive: row["LinkSpeedActv"],
			Raw:             cloneRow(row),
		})
	}

	return ports, nil
}

func parseNodeInfos(rows []map[string]string) ([]models.NodeInfo, error) {
	infos := make([]models.NodeInfo, 0, len(rows))

	for i, row := range rows {
		nodeGUID := strings.TrimSpace(row["NodeGuid"])
		if nodeGUID == "" {
			return nil, fmt.Errorf("node info row %d: NodeGuid is required", i+1)
		}

		infos = append(infos, models.NodeInfo{
			NodeGUID:     nodeGUID,
			SerialNumber: row["SerialNumber"],
			PartNumber:   row["PartNumber"],
			Revision:     row["Revision"],
			ProductName:  row["ProductName"],
			Raw:          cloneRow(row),
		})
	}

	return infos, nil
}

func validateResult(result *models.ParseResult) error {
	if len(result.Nodes) == 0 {
		return errors.New("nodes section has no rows")
	}
	if len(result.Ports) == 0 {
		return errors.New("ports section has no rows")
	}

	nodesByGUID := make(map[string]struct{}, len(result.Nodes))
	for _, node := range result.Nodes {
		if _, exists := nodesByGUID[node.NodeGUID]; exists {
			return fmt.Errorf("duplicated node guid %s", node.NodeGUID)
		}
		nodesByGUID[node.NodeGUID] = struct{}{}
	}

	for _, port := range result.Ports {
		if _, exists := nodesByGUID[port.NodeGUID]; !exists {
			return fmt.Errorf("port %s:%d references unknown node", port.NodeGUID, port.PortNum)
		}
	}

	for _, info := range result.NodeInfos {
		if _, exists := nodesByGUID[info.NodeGUID]; !exists {
			return fmt.Errorf("node info references unknown node %s", info.NodeGUID)
		}
	}

	return nil
}

func normalizeNodeType(value string) string {
	switch strings.TrimSpace(value) {
	case "1":
		return "host"
	case "2":
		return "switch"
	default:
		return "unknown"
	}
}

func normalizePortState(code int) string {
	switch code {
	case 0:
		return "unknown"
	case 1:
		return "down"
	case 2:
		return "init"
	case 3:
		return "armed"
	case 4:
		return "active"
	default:
		return "unknown"
	}
}

func parseInt(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}

func cloneRow(row map[string]string) map[string]string {
	clone := make(map[string]string, len(row))
	for key, value := range row {
		clone[key] = value
	}

	return clone
}
