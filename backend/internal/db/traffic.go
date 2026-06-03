package db

import "time"

func (s *Store) ReplaceUserTraffic(entries []UserTraffic) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_traffic"); err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, e := range entries {
		_, err := tx.Exec(`
			INSERT INTO user_traffic (email, server_id, up, down, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, e.Email, e.ServerID, e.Up, e.Down, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ReplaceUserTrafficForServer(serverID int64, entries []UserTraffic) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM user_traffic WHERE server_id = ?", serverID); err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, e := range entries {
		_, err := tx.Exec(`
			INSERT INTO user_traffic (email, server_id, up, down, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, e.Email, e.ServerID, e.Up, e.Down, now)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetUserTrafficByEmail(email string) ([]UserTraffic, error) {
	rows, err := s.DB.Query(`
		SELECT id, email, server_id, up, down, updated_at
		FROM user_traffic WHERE email = ?
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserTraffic, 0)
	for rows.Next() {
		var t UserTraffic
		if err := rows.Scan(&t.ID, &t.Email, &t.ServerID, &t.Up, &t.Down, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetAllUserTraffic() ([]UserTraffic, error) {
	rows, err := s.DB.Query(`
		SELECT id, email, server_id, up, down, updated_at
		FROM user_traffic ORDER BY email
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserTraffic, 0)
	for rows.Next() {
		var t UserTraffic
		if err := rows.Scan(&t.ID, &t.Email, &t.ServerID, &t.Up, &t.Down, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ZeroUserTrafficForServer(serverID int64, emails []string) error {
	if len(emails) == 0 {
		return nil
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	stmt, err := tx.Prepare(`
		UPDATE user_traffic
		SET up = 0, down = 0, updated_at = ?
		WHERE server_id = ? AND email = ?
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, email := range emails {
		if email == "" {
			continue
		}
		if _, err := stmt.Exec(now, serverID, email); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ZeroAllUserTrafficForServer(serverID int64) error {
	_, err := s.DB.Exec(`
		UPDATE user_traffic
		SET up = 0, down = 0, updated_at = ?
		WHERE server_id = ?
	`, time.Now().Unix(), serverID)
	return err
}
