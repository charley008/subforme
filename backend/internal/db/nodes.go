package db

import (
	"fmt"
	"strings"
	"time"
)

func (s *Store) ListNodeDB() ([]Node2, error) {
	rows, err := s.DB.Query(`
		SELECT id, node_id, name, address, COALESCE(port, 443),
		       COALESCE(protocol, 'vless'), COALESCE(network, 'raw'),
		       COALESCE(flow, ''), COALESCE(server_name, ''), COALESCE(server_id, 0)
		FROM nodes ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Node2, 0)
	for rows.Next() {
		var n Node2
		if err := rows.Scan(&n.ID, &n.NodeID, &n.Name, &n.Address, &n.Port, &n.Protocol, &n.Network, &n.Flow, &n.ServerName, &n.ServerID); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) ReplaceNodes(nodes []Node2) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM nodes"); err != nil {
		return err
	}
	if err := saveJSONSettingTx(tx, nodesKey, map[string]bool{"seeded": true}); err != nil {
		return err
	}

	for _, n := range nodes {
		nid := n.NodeID
		if nid == "" {
			nid = fmt.Sprintf("node-%d", time.Now().UnixNano())
		}
		if n.Protocol == "" {
			n.Protocol = "vless"
		}
		if n.Network == "" {
			n.Network = "raw"
		}
		n.Flow = strings.TrimSpace(n.Flow)
		n.ServerName = strings.TrimSpace(n.ServerName)
		_, err := tx.Exec(`
			INSERT INTO nodes (node_id, name, address, port, protocol, network, flow, server_name, server_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nid, n.Name, n.Address, n.Port, n.Protocol, n.Network, n.Flow, n.ServerName, n.ServerID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
