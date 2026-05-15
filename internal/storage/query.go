package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log-parser/internal/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetLog(ctx context.Context, logID string) (*models.Log, error) {
	logRecord := &models.Log{}

	err := s.pool.QueryRow(ctx, `
		SELECT id::text, file_path, status, error_message, nodes_count, ports_count, uploaded_at, parsed_at
		FROM logs
		WHERE id = $1
	`, logID).Scan(
		&logRecord.ID,
		&logRecord.FilePath,
		&logRecord.Status,
		&logRecord.ErrorMessage,
		&logRecord.NodesCount,
		&logRecord.PortsCount,
		&logRecord.UploadedAt,
		&logRecord.ParsedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get log: %w", err)
	}

	return logRecord, nil
}

func (s *Store) GetTopology(ctx context.Context, logID string) (*models.Topology, error) {
	if _, err := s.GetLog(ctx, logID); err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT n.id::text, n.external_id, n.name, n.type, n.num_ports, count(p.id)::int
		FROM nodes n
		LEFT JOIN ports p ON p.node_id = n.id
		WHERE n.log_id = $1
		GROUP BY n.id, n.external_id, n.name, n.type, n.num_ports
		ORDER BY n.type, n.name
	`, logID)
	if err != nil {
		return nil, fmt.Errorf("query topology nodes: %w", err)
	}
	defer rows.Close()

	topology := &models.Topology{
		LogID: logID,
		Links: []models.TopologyLink{},
	}
	groups := map[string][]string{}

	for rows.Next() {
		var node models.TopologyNode
		if err := rows.Scan(&node.ID, &node.ExternalID, &node.Name, &node.Type, &node.NumPorts, &node.PortsCount); err != nil {
			return nil, fmt.Errorf("scan topology node: %w", err)
		}

		topology.Nodes = append(topology.Nodes, node)
		groups[node.Type] = append(groups[node.Type], node.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topology nodes: %w", err)
	}

	for _, groupType := range []string{"host", "switch", "unknown"} {
		nodeIDs := groups[groupType]
		if len(nodeIDs) == 0 {
			continue
		}

		topology.Groups = append(topology.Groups, models.TopologyGroup{
			Type:    groupType,
			NodeIDs: nodeIDs,
			Count:   len(nodeIDs),
		})
	}

	return topology, nil
}

func (s *Store) GetNode(ctx context.Context, nodeID string) (*models.Node, *models.NodeInfo, error) {
	node := &models.Node{}
	var rawData []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id::text, log_id::text, external_id, name, type, num_ports, node_guid, raw
		FROM nodes
		WHERE id = $1
	`, nodeID).Scan(
		&node.ID,
		&node.LogID,
		&node.ExternalID,
		&node.Name,
		&node.Type,
		&node.NumPorts,
		&node.NodeGUID,
		&rawData,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, models.ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get node: %w", err)
	}

	raw, err := unmarshalRaw(rawData)
	if err != nil {
		return nil, nil, fmt.Errorf("unmarshal node raw: %w", err)
	}
	node.Raw = raw

	info, err := s.getNodeInfo(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}

	return node, info, nil
}

func (s *Store) GetPortsByNodeID(ctx context.Context, nodeID string) ([]models.Port, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM nodes WHERE id = $1)`, nodeID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check node exists: %w", err)
	}
	if !exists {
		return nil, models.ErrNotFound
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			id::text, log_id::text, node_id::text, node_guid, port_guid, port_num, lid,
			port_phy_state, port_state_code, port_state, link_width_active, link_speed_active, raw
		FROM ports
		WHERE node_id = $1
		ORDER BY port_num
	`, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query ports: %w", err)
	}
	defer rows.Close()

	ports := []models.Port{}
	for rows.Next() {
		var port models.Port
		var rawData []byte

		if err := rows.Scan(
			&port.ID,
			&port.LogID,
			&port.NodeID,
			&port.NodeGUID,
			&port.PortGUID,
			&port.PortNum,
			&port.LID,
			&port.PortPhyState,
			&port.PortStateCode,
			&port.PortState,
			&port.LinkWidthActive,
			&port.LinkSpeedActive,
			&rawData,
		); err != nil {
			return nil, fmt.Errorf("scan port: %w", err)
		}

		raw, err := unmarshalRaw(rawData)
		if err != nil {
			return nil, fmt.Errorf("unmarshal port raw: %w", err)
		}
		port.Raw = raw

		ports = append(ports, port)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ports: %w", err)
	}

	return ports, nil
}

func (s *Store) getNodeInfo(ctx context.Context, nodeID string) (*models.NodeInfo, error) {
	info := &models.NodeInfo{}
	var rawData []byte

	err := s.pool.QueryRow(ctx, `
		SELECT
			id::text, log_id::text, node_id::text, node_guid, serial_number,
			part_number, revision, product_name, raw
		FROM nodes_info
		WHERE node_id = $1
	`, nodeID).Scan(
		&info.ID,
		&info.LogID,
		&info.NodeID,
		&info.NodeGUID,
		&info.SerialNumber,
		&info.PartNumber,
		&info.Revision,
		&info.ProductName,
		&rawData,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get node info: %w", err)
	}

	raw, err := unmarshalRaw(rawData)
	if err != nil {
		return nil, fmt.Errorf("unmarshal node info raw: %w", err)
	}
	info.Raw = raw

	return info, nil
}

func unmarshalRaw(data []byte) (map[string]string, error) {
	raw := map[string]string{}
	if len(data) == 0 {
		return raw, nil
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return raw, nil
}
