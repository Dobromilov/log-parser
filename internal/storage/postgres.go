package storage

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log-parser/internal/models"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) SaveParseResult(ctx context.Context, filePath string, result *models.ParseResult) (*models.Log, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	logID, err := newUUID()
	if err != nil {
		return nil, err
	}

	parsedAt := time.Now().UTC()
	logRecord := &models.Log{
		ID:         logID,
		FilePath:   filePath,
		Status:     "parsed",
		NodesCount: len(result.Nodes),
		PortsCount: len(result.Ports),
		ParsedAt:   &parsedAt,
	}

	if err := tx.QueryRow(ctx, `
		INSERT INTO logs (id, file_path, status, nodes_count, ports_count, parsed_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING uploaded_at
	`, logRecord.ID, logRecord.FilePath, logRecord.Status, logRecord.NodesCount, logRecord.PortsCount, logRecord.ParsedAt).Scan(&logRecord.UploadedAt); err != nil {
		return nil, fmt.Errorf("insert log: %w", err)
	}

	nodeIDs := make(map[string]string, len(result.Nodes))
	for _, node := range result.Nodes {
		nodeID, err := newUUID()
		if err != nil {
			return nil, err
		}

		raw, err := marshalRaw(node.Raw)
		if err != nil {
			return nil, fmt.Errorf("marshal node raw: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO nodes (id, log_id, external_id, name, type, num_ports, node_guid, raw)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
		`, nodeID, logRecord.ID, node.ExternalID, node.Name, node.Type, node.NumPorts, node.NodeGUID, raw); err != nil {
			return nil, fmt.Errorf("insert node %s: %w", node.NodeGUID, err)
		}

		nodeIDs[node.NodeGUID] = nodeID
	}

	for _, port := range result.Ports {
		nodeID, ok := nodeIDs[port.NodeGUID]
		if !ok {
			return nil, fmt.Errorf("port references unknown node %s", port.NodeGUID)
		}

		portID, err := newUUID()
		if err != nil {
			return nil, err
		}

		raw, err := marshalRaw(port.Raw)
		if err != nil {
			return nil, fmt.Errorf("marshal port raw: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO ports (
				id, log_id, node_id, node_guid, port_guid, port_num, lid,
				port_phy_state, port_state_code, port_state, link_width_active,
				link_speed_active, raw
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb)
		`, portID, logRecord.ID, nodeID, port.NodeGUID, port.PortGUID, port.PortNum, port.LID, port.PortPhyState, port.PortStateCode, port.PortState, port.LinkWidthActive, port.LinkSpeedActive, raw); err != nil {
			return nil, fmt.Errorf("insert port %s:%d: %w", port.NodeGUID, port.PortNum, err)
		}
	}

	for _, info := range result.NodeInfos {
		nodeID, ok := nodeIDs[info.NodeGUID]
		if !ok {
			return nil, fmt.Errorf("node info references unknown node %s", info.NodeGUID)
		}

		infoID, err := newUUID()
		if err != nil {
			return nil, err
		}

		raw, err := marshalRaw(info.Raw)
		if err != nil {
			return nil, fmt.Errorf("marshal node info raw: %w", err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO nodes_info (
				id, log_id, node_id, node_guid, serial_number,
				part_number, revision, product_name, raw
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		`, infoID, logRecord.ID, nodeID, info.NodeGUID, info.SerialNumber, info.PartNumber, info.Revision, info.ProductName, raw); err != nil {
			return nil, fmt.Errorf("insert node info %s: %w", info.NodeGUID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return logRecord, nil
}

func marshalRaw(raw map[string]string) (string, error) {
	if raw == nil {
		raw = map[string]string{}
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func newUUID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16]), nil
}
