package db

import (
	"fmt"
	"time"
)

func (s *Store) UpsertInbound(inb *Inbound) error {
	inb.UpdatedAt = time.Now().Unix()
	_, err := s.DB.Exec(`
		INSERT INTO inbounds (server_id, inbound_id, remark, port, protocol,
		                      settings_json, stream_settings_json, sniffing_json,
		                      tag, enable, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(server_id, inbound_id) DO UPDATE SET
			remark=excluded.remark, port=excluded.port, protocol=excluded.protocol,
			settings_json=excluded.settings_json,
			stream_settings_json=excluded.stream_settings_json,
			sniffing_json=excluded.sniffing_json,
			tag=excluded.tag, enable=excluded.enable, updated_at=excluded.updated_at
	`, inb.ServerID, inb.InboundID, inb.Remark, inb.Port, inb.Protocol,
		inb.SettingsJSON, inb.StreamSettingsJSON, inb.SniffingJSON,
		inb.Tag, boolToInt(inb.Enable), inb.UpdatedAt)
	return err
}

func (s *Store) ListInboundsByServer(serverID int64) ([]Inbound, error) {
	rows, err := s.DB.Query(`
		SELECT id, server_id, inbound_id, remark, port, protocol,
		       settings_json, COALESCE(stream_settings_json,''),
		       COALESCE(sniffing_json,''), COALESCE(tag,''), enable,
		       COALESCE(traffic_json,''), updated_at
		FROM inbounds WHERE server_id = ? ORDER BY remark
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Inbound, 0)
	for rows.Next() {
		var in Inbound
		if err := rows.Scan(&in.ID, &in.ServerID, &in.InboundID, &in.Remark,
			&in.Port, &in.Protocol, &in.SettingsJSON, &in.StreamSettingsJSON,
			&in.SniffingJSON, &in.Tag, &in.Enable, &in.TrafficJSON, &in.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

func (s *Store) GetInbound(id int64) (*Inbound, error) {
	var in Inbound
	err := s.DB.QueryRow(`
		SELECT id, server_id, inbound_id, remark, port, protocol,
		       settings_json, COALESCE(stream_settings_json,''),
		       COALESCE(sniffing_json,''), COALESCE(tag,''), enable,
		       COALESCE(traffic_json,''), updated_at
		FROM inbounds WHERE id = ?
	`, id).Scan(&in.ID, &in.ServerID, &in.InboundID, &in.Remark,
		&in.Port, &in.Protocol, &in.SettingsJSON, &in.StreamSettingsJSON,
		&in.SniffingJSON, &in.Tag, &in.Enable, &in.TrafficJSON, &in.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &in, nil
}

func (s *Store) DeleteInboundsByServer(serverID int64) error {
	_, err := s.DB.Exec("DELETE FROM inbounds WHERE server_id = ?", serverID)
	return err
}

// UpdateInboundClientsJSON updates only the settings_json (clients array) for an inbound.
// Used when adding/removing clients via 3x-ui API to keep local cache in sync.
func (s *Store) UpdateInboundClientsJSON(id int64, settingsJSON string) error {
	now := time.Now().Unix()
	_, err := s.DB.Exec("UPDATE inbounds SET settings_json=?, updated_at=? WHERE id=?", settingsJSON, now, id)
	return err
}

// FindInboundsByServerAndProtocol returns enabled inbounds on a server matching the given protocol.
func (s *Store) FindInboundsByServerAndProtocol(serverID int64, protocol string) ([]Inbound, error) {
	rows, err := s.DB.Query(`
		SELECT id, server_id, inbound_id, remark, port, protocol,
		       settings_json, COALESCE(stream_settings_json,''),
		       COALESCE(sniffing_json,''), COALESCE(tag,''), enable,
		       COALESCE(traffic_json,''), updated_at
		FROM inbounds WHERE server_id = ? AND protocol = ? AND enable = 1
		ORDER BY remark
	`, serverID, protocol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Inbound, 0)
	for rows.Next() {
		var in Inbound
		if err := rows.Scan(&in.ID, &in.ServerID, &in.InboundID, &in.Remark,
			&in.Port, &in.Protocol, &in.SettingsJSON, &in.StreamSettingsJSON,
			&in.SniffingJSON, &in.Tag, &in.Enable, &in.TrafficJSON, &in.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, rows.Err()
}

// EnsureServerInbounds syncs inbounds from 3x-ui into local cache.
// It deletes old inbounds for the server and inserts current ones.
func (s *Store) EnsureServerInbounds(serverID int64, inbounds []Inbound) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM inbounds WHERE server_id = ?", serverID); err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, in := range inbounds {
		in.ServerID = serverID
		in.UpdatedAt = now
		_, err := tx.Exec(`
			INSERT INTO inbounds (server_id, inbound_id, remark, port, protocol,
			                      settings_json, stream_settings_json, sniffing_json,
			                      tag, enable, traffic_json, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, in.ServerID, in.InboundID, in.Remark, in.Port, in.Protocol,
			in.SettingsJSON, in.StreamSettingsJSON, in.SniffingJSON,
			in.Tag, boolToInt(in.Enable), in.TrafficJSON, in.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert inbound: %w", err)
		}
	}
	return tx.Commit()
}
