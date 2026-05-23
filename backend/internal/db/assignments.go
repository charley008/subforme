package db

import "time"

func (s *Store) CreateAssignment(a *UserAssignment) error {
	now := time.Now().Unix()
	res, err := s.DB.Exec(`
		INSERT INTO user_assignments (user_id, server_id, inbound_id, email_on_server, enable, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, server_id, inbound_id) DO NOTHING
	`, a.UserID, a.ServerID, a.InboundID, a.EmailOnServer, boolToInt(a.Enable), now)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	a.ID = id
	a.CreatedAt = now
	return nil
}

func (s *Store) DeleteAssignment(userID, serverID, inboundID int64) error {
	_, err := s.DB.Exec(
		"DELETE FROM user_assignments WHERE user_id=? AND server_id=? AND inbound_id=?",
		userID, serverID, inboundID)
	return err
}

func (s *Store) DeleteAssignmentsByUser(userID int64) error {
	_, err := s.DB.Exec("DELETE FROM user_assignments WHERE user_id=?", userID)
	return err
}

func (s *Store) DeleteAssignmentsByServer(serverID int64) error {
	_, err := s.DB.Exec("DELETE FROM user_assignments WHERE server_id=?", serverID)
	return err
}

func (s *Store) ListAssignmentsByUser(userID int64) ([]UserAssignment, error) {
	rows, err := s.DB.Query(`
		SELECT id, user_id, server_id, inbound_id, email_on_server, enable, created_at
		FROM user_assignments WHERE user_id = ? ORDER BY server_id, inbound_id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserAssignment, 0)
	for rows.Next() {
		var a UserAssignment
		if err := rows.Scan(&a.ID, &a.UserID, &a.ServerID, &a.InboundID,
			&a.EmailOnServer, &a.Enable, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListAssignmentsByServer(serverID int64) ([]UserAssignment, error) {
	rows, err := s.DB.Query(`
		SELECT id, user_id, server_id, inbound_id, email_on_server, enable, created_at
		FROM user_assignments WHERE server_id = ? ORDER BY user_id
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]UserAssignment, 0)
	for rows.Next() {
		var a UserAssignment
		if err := rows.Scan(&a.ID, &a.UserID, &a.ServerID, &a.InboundID,
			&a.EmailOnServer, &a.Enable, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}

	return out, rows.Err()
}

// FindAssignment returns a specific assignment by user+server+inbound.
func (s *Store) FindAssignment(userID, serverID, inboundID int64) (*UserAssignment, error) {
	var a UserAssignment
	err := s.DB.QueryRow(`
		SELECT id, user_id, server_id, inbound_id, email_on_server, enable, created_at
		FROM user_assignments WHERE user_id=? AND server_id=? AND inbound_id=?
	`, userID, serverID, inboundID).Scan(&a.ID, &a.UserID, &a.ServerID, &a.InboundID,
		&a.EmailOnServer, &a.Enable, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
