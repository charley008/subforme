package db

import (
	"fmt"
	"time"
)

func (s *Store) ListNodeDB() ([]Node2, error) {
	rows, err := s.DB.Query(`
		SELECT id, node_id, name, address, COALESCE(port, 443), COALESCE(server_id, 0)
		FROM nodes ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Node2, 0)
	for rows.Next() {
		var n Node2
		if err := rows.Scan(&n.ID, &n.NodeID, &n.Name, &n.Address, &n.Port, &n.ServerID); err != nil {
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
		_, err := tx.Exec(`
			INSERT INTO nodes (node_id, name, address, port, server_id)
			VALUES (?, ?, ?, ?, ?)
		`, nid, n.Name, n.Address, n.Port, n.ServerID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
