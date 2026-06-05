package db

import (
	"database/sql"
	"time"
)

func (s *Store) CreateUser(u *User) error {
	now := time.Now().Unix()
	res, err := s.DB.Exec(`
		INSERT INTO users (email, uuid, password, auth, flow, security, remark,
		                   total_gb, expiry_time, limit_ip, sub_id, tg_id,
		                   reset, comment, enable, mode, node_ids_json,
		                   provider_ids_json, group_nodes_json, group_modes_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, u.Email, u.UUID, u.Password, u.Auth, u.Flow, u.Security, u.Remark,
		u.TotalGB, u.ExpiryTime, u.LimitIP, u.SubID, u.TgID,
		u.Reset, u.Comment, boolToInt(u.Enable), u.Mode, u.NodeIDsJSON,
		u.ProviderIDsJSON, u.GroupNodesJSON, u.GroupModesJSON, now, now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	u.CreatedAt = now
	u.UpdatedAt = now
	return nil
}

func (s *Store) UpdateUser(u *User) error {
	if current, err := s.GetUser(u.ID); err == nil {
		if u.Mode == "" {
			u.Mode = current.Mode
		}
		if u.NodeIDsJSON == "" {
			u.NodeIDsJSON = current.NodeIDsJSON
		}
		if u.ProviderIDsJSON == "" {
			u.ProviderIDsJSON = current.ProviderIDsJSON
		}
		if u.GroupNodesJSON == "" {
			u.GroupNodesJSON = current.GroupNodesJSON
		}
		if u.GroupModesJSON == "" {
			u.GroupModesJSON = current.GroupModesJSON
		}
	}
	u.UpdatedAt = time.Now().Unix()
	_, err := s.DB.Exec(`
		UPDATE users SET email=?, uuid=?, password=?, auth=?, flow=?, security=?,
		       remark=?, total_gb=?, expiry_time=?, limit_ip=?, sub_id=?,
		       tg_id=?, reset=?, comment=?, enable=?, mode=?,
		       node_ids_json=?, provider_ids_json=?, group_nodes_json=?, group_modes_json=?,
		       updated_at=?
		WHERE id=?
	`, u.Email, u.UUID, u.Password, u.Auth, u.Flow, u.Security, u.Remark,
		u.TotalGB, u.ExpiryTime, u.LimitIP, u.SubID, u.TgID,
		u.Reset, u.Comment, boolToInt(u.Enable), u.Mode, u.NodeIDsJSON,
		u.ProviderIDsJSON, u.GroupNodesJSON, u.GroupModesJSON, u.UpdatedAt, u.ID)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_assignments WHERE user_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.DB.Query(`
		SELECT id, email, uuid, COALESCE(password,''), COALESCE(auth,''),
		       COALESCE(flow,''), COALESCE(security,''), COALESCE(remark,''),
		       total_gb, expiry_time, limit_ip, COALESCE(sub_id,''),
		       COALESCE(tg_id,0), COALESCE(reset,0), COALESCE(comment,''),
		       enable, COALESCE(mode,''), COALESCE(node_ids_json,''),
		       COALESCE(provider_ids_json,''), COALESCE(group_nodes_json,''), COALESCE(group_modes_json,''),
		       created_at, updated_at
		FROM users ORDER BY email
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsers(rows)
}

func (s *Store) ListEnabledUsers() ([]User, error) {
	rows, err := s.DB.Query(`
		SELECT id, email, uuid, COALESCE(password,''), COALESCE(auth,''),
		       COALESCE(flow,''), COALESCE(security,''), COALESCE(remark,''),
		       total_gb, expiry_time, limit_ip, COALESCE(sub_id,''),
		       COALESCE(tg_id,0), COALESCE(reset,0), COALESCE(comment,''),
		       enable, COALESCE(mode,''), COALESCE(node_ids_json,''),
		       COALESCE(provider_ids_json,''), COALESCE(group_nodes_json,''), COALESCE(group_modes_json,''),
		       created_at, updated_at
		FROM users WHERE enable = 1 ORDER BY email
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsers(rows)
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	var u User
	err := s.DB.QueryRow(`
		SELECT id, email, uuid, COALESCE(password,''), COALESCE(auth,''),
		       COALESCE(flow,''), COALESCE(security,''), COALESCE(remark,''),
		       total_gb, expiry_time, limit_ip, COALESCE(sub_id,''),
		       COALESCE(tg_id,0), COALESCE(reset,0), COALESCE(comment,''),
		       enable, COALESCE(mode,''), COALESCE(node_ids_json,''),
		       COALESCE(provider_ids_json,''), COALESCE(group_nodes_json,''), COALESCE(group_modes_json,''),
		       created_at, updated_at
	FROM users WHERE email = ?
	`, email).Scan(&u.ID, &u.Email, &u.UUID, &u.Password, &u.Auth,
		&u.Flow, &u.Security, &u.Remark, &u.TotalGB, &u.ExpiryTime,
		&u.LimitIP, &u.SubID, &u.TgID, &u.Reset, &u.Comment,
		&u.Enable, &u.Mode, &u.NodeIDsJSON, &u.ProviderIDsJSON,
		&u.GroupNodesJSON, &u.GroupModesJSON, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUser(id int64) (*User, error) {
	var u User
	err := s.DB.QueryRow(`
		SELECT id, email, uuid, COALESCE(password,''), COALESCE(auth,''),
		       COALESCE(flow,''), COALESCE(security,''), COALESCE(remark,''),
		       total_gb, expiry_time, limit_ip, COALESCE(sub_id,''),
		       COALESCE(tg_id,0), COALESCE(reset,0), COALESCE(comment,''),
		       enable, COALESCE(mode,''), COALESCE(node_ids_json,''),
		       COALESCE(provider_ids_json,''), COALESCE(group_nodes_json,''), COALESCE(group_modes_json,''),
		       created_at, updated_at
	FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Email, &u.UUID, &u.Password, &u.Auth,
		&u.Flow, &u.Security, &u.Remark, &u.TotalGB, &u.ExpiryTime,
		&u.LimitIP, &u.SubID, &u.TgID, &u.Reset, &u.Comment,
		&u.Enable, &u.Mode, &u.NodeIDsJSON, &u.ProviderIDsJSON,
		&u.GroupNodesJSON, &u.GroupModesJSON, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) SearchUsers(query string) ([]User, error) {
	like := "%" + query + "%"
	rows, err := s.DB.Query(`
		SELECT id, email, uuid, COALESCE(password,''), COALESCE(auth,''),
		       COALESCE(flow,''), COALESCE(security,''), COALESCE(remark,''),
		       total_gb, expiry_time, limit_ip, COALESCE(sub_id,''),
		       COALESCE(tg_id,0), COALESCE(reset,0), COALESCE(comment,''),
		       enable, COALESCE(mode,''), COALESCE(node_ids_json,''),
		       COALESCE(provider_ids_json,''), COALESCE(group_nodes_json,''), COALESCE(group_modes_json,''),
		       created_at, updated_at
		FROM users WHERE email LIKE ? OR COALESCE(remark,'') LIKE ?
		ORDER BY email LIMIT 50
	`, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanUsers(rows)
}

func scanUsers(rows *sql.Rows) ([]User, error) {
	out := make([]User, 0)
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.UUID, &u.Password, &u.Auth,
			&u.Flow, &u.Security, &u.Remark, &u.TotalGB, &u.ExpiryTime,
			&u.LimitIP, &u.SubID, &u.TgID, &u.Reset, &u.Comment,
			&u.Enable, &u.Mode, &u.NodeIDsJSON, &u.ProviderIDsJSON,
			&u.GroupNodesJSON, &u.GroupModesJSON, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
