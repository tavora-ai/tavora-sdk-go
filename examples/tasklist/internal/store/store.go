// Package store is the SQLite-backed persistence for task lists and tasks.
package store

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type TaskList struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	TaskCount   int       `json:"task_count"`
}

type Task struct {
	ID        string    `json:"id"`
	ListID    string    `json:"list_id"`
	Title     string    `json:"title"`
	Notes     string    `json:"notes"`
	DueDate   string    `json:"due_date"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

// Open opens a SQLite database at dsn and applies the schema. Pass
// ":memory:" for an in-memory store (tests).
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) CreateList(name, description string) (*TaskList, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	l := &TaskList{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	_, err := s.db.Exec(
		`INSERT INTO task_lists (id, name, description, created_at) VALUES (?, ?, ?, ?)`,
		l.ID, l.Name, l.Description, l.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert list: %w", err)
	}
	return l, nil
}

func (s *Store) ListLists() ([]TaskList, error) {
	rows, err := s.db.Query(`
		SELECT l.id, l.name, l.description, l.created_at,
		       (SELECT COUNT(*) FROM tasks t WHERE t.list_id = l.id) AS task_count
		FROM task_lists l
		ORDER BY l.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TaskList{}
	for rows.Next() {
		var l TaskList
		if err := rows.Scan(&l.ID, &l.Name, &l.Description, &l.CreatedAt, &l.TaskCount); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) DeleteList(id string) error {
	res, err := s.db.Exec(`DELETE FROM task_lists WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("list not found")
	}
	return nil
}

func (s *Store) AddTask(listID, title, notes, dueDate string) (*Task, error) {
	if listID == "" || title == "" {
		return nil, errors.New("list_id and title are required")
	}
	var exists int
	if err := s.db.QueryRow(`SELECT 1 FROM task_lists WHERE id = ?`, listID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("list not found")
		}
		return nil, err
	}
	t := &Task{
		ID:        uuid.NewString(),
		ListID:    listID,
		Title:     title,
		Notes:     notes,
		DueDate:   dueDate,
		CreatedAt: time.Now().UTC(),
	}
	_, err := s.db.Exec(
		`INSERT INTO tasks (id, list_id, title, notes, due_date, done, created_at) VALUES (?, ?, ?, ?, ?, 0, ?)`,
		t.ID, t.ListID, t.Title, t.Notes, t.DueDate, t.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	return t, nil
}

func (s *Store) ListTasks(listID string) ([]Task, error) {
	rows, err := s.db.Query(`
		SELECT id, list_id, title, notes, due_date, done, created_at
		FROM tasks WHERE list_id = ? ORDER BY done ASC, created_at ASC
	`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		var done int
		if err := rows.Scan(&t.ID, &t.ListID, &t.Title, &t.Notes, &t.DueDate, &done, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Done = done != 0
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) CompleteTask(id string) (*Task, error) {
	res, err := s.db.Exec(`UPDATE tasks SET done = 1 WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, errors.New("task not found")
	}
	var t Task
	var done int
	err = s.db.QueryRow(
		`SELECT id, list_id, title, notes, due_date, done, created_at FROM tasks WHERE id = ?`, id,
	).Scan(&t.ID, &t.ListID, &t.Title, &t.Notes, &t.DueDate, &done, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	t.Done = done != 0
	return &t, nil
}

func (s *Store) DeleteTask(id string) error {
	res, err := s.db.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("task not found")
	}
	return nil
}
