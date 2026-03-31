package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// User represents a user account.
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateUser inserts a new user.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash, role string) (*User, error) {
	now := time.Now().UTC()
	result, err := s.writeDB.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		username, passwordHash, role, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	id, _ := result.LastInsertId()
	return &User{
		ID:       int(id),
		Username: username,
		Role:     role,
		CreatedAt: now,
	}, nil
}

// GetUserByUsername fetches a user by username (includes password hash for auth).
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	var u User
	err := s.readDB.QueryRowContext(ctx,
		`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &u, nil
}

// ListUsers returns all users (without password hashes).
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT id, username, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	return users, rows.Err()
}

// UpdateUserRole changes a user's role.
func (s *Store) UpdateUserRole(ctx context.Context, username, role string) error {
	result, err := s.writeDB.ExecContext(ctx,
		`UPDATE users SET role = ? WHERE username = ?`, role, username)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteUser removes a user.
func (s *Store) DeleteUser(ctx context.Context, username string) error {
	result, err := s.writeDB.ExecContext(ctx,
		`DELETE FROM users WHERE username = ?`, username)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// UserCount returns the number of users.
func (s *Store) UserCount(ctx context.Context) (int, error) {
	var count int
	err := s.readDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}
