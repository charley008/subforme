package db

import (
	"fmt"
	"time"
)

func (s *Store) CreateServer(sv *Server) error {
	now := time.Now().Unix()
	res, err := s.DB.Exec(`
		INSERT INTO servers (name, scheme, host, port, base_path, api_key,
		                     sub_address, sub_port, is_main, remark, enabled,
		                     created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, sv.Name, sv.Scheme, sv.Host, sv.Port, sv.BasePath, sv.APIKey,
		sv.SubAddress, sv.SubPort, boolToInt(sv.IsMain), sv.Remark,
		boolToInt(sv.Enabled), now, now)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	id, _ := res.LastInsertId()
	sv.ID = id
	sv.CreatedAt = now
	sv.UpdatedAt = now
	return nil
}

func (s *Store) UpdateServer(sv *Server) error {
	sv.UpdatedAt = time.Now().Unix()
	_, err := s.DB.Exec(`
		UPDATE servers SET name=?, scheme=?, host=?, port=?, base_path=?,
		       api_key=?, sub_address=?, sub_port=?, is_main=?, remark=?,
		       enabled=?, updated_at=?
		WHERE id=?
	`, sv.Name, sv.Scheme, sv.Host, sv.Port, sv.BasePath, sv.APIKey,
		sv.SubAddress, sv.SubPort, boolToInt(sv.IsMain), sv.Remark,
		boolToInt(sv.Enabled), sv.UpdatedAt, sv.ID)
	return err
}

func (s *Store) DeleteServer(id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_assignments WHERE server_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM inbounds WHERE server_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM servers WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListServers() ([]Server, error) {
	rows, err := s.DB.Query(`
		SELECT id, name, scheme, host, port, base_path, api_key,
		       COALESCE(sub_address,''), COALESCE(sub_port,0),
		       is_main, COALESCE(remark,''), enabled, created_at, updated_at
		FROM servers ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Server, 0)
	for rows.Next() {
		var sv Server
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.Scheme, &sv.Host, &sv.Port,
			&sv.BasePath, &sv.APIKey, &sv.SubAddress, &sv.SubPort,
			&sv.IsMain, &sv.Remark, &sv.Enabled, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) GetServer(id int64) (*Server, error) {
	var sv Server
	err := s.DB.QueryRow(`
		SELECT id, name, scheme, host, port, base_path, api_key,
		       COALESCE(sub_address,''), COALESCE(sub_port,0),
		       is_main, COALESCE(remark,''), enabled, created_at, updated_at
		FROM servers WHERE id = ?
	`, id).Scan(&sv.ID, &sv.Name, &sv.Scheme, &sv.Host, &sv.Port,
		&sv.BasePath, &sv.APIKey, &sv.SubAddress, &sv.SubPort,
		&sv.IsMain, &sv.Remark, &sv.Enabled, &sv.CreatedAt, &sv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sv, nil
}

func (s *Store) GetMainServer() (*Server, error) {
	var sv Server
	err := s.DB.QueryRow(`
		SELECT id, name, scheme, host, port, base_path, api_key,
		       COALESCE(sub_address,''), COALESCE(sub_port,0),
		       is_main, COALESCE(remark,''), enabled, created_at, updated_at
		FROM servers WHERE is_main = 1 AND enabled = 1 LIMIT 1
	`).Scan(&sv.ID, &sv.Name, &sv.Scheme, &sv.Host, &sv.Port,
		&sv.BasePath, &sv.APIKey, &sv.SubAddress, &sv.SubPort,
		&sv.IsMain, &sv.Remark, &sv.Enabled, &sv.CreatedAt, &sv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sv, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
