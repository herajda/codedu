package main

import (
	"fmt"
	"time"
)

type User struct {
	ID           int       `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	CreatedAt    time.Time `db:"created_at"`
}

type Assignment struct {
	ID          int       `db:"id" json:"id"`
	Title       string    `db:"title" json:"title"`
	Description string    `db:"description" json:"description"`
	CreatedBy   int       `db:"created_by" json:"created_by"`
	Deadline    time.Time `db:"deadline" json:"deadline"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

func CreateStudent(email, hash string) error {
	_, err := DB.Exec(
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, 'student')`,
		email, hash,
	)
	return err
}

func FindUserByEmail(email string) (*User, error) {
	var u User
	// only the columns in your User struct
	err := DB.Get(&u,
		`SELECT id, email, password_hash, role
       FROM users
      WHERE email = $1`,
		email,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateAssignment inserts a new assignment and returns its ID.
func CreateAssignment(a *Assignment) error {
	query := `
    INSERT INTO assignments (title, description, created_by, deadline)
    VALUES ($1, $2, $3, $4)
    RETURNING id, created_at, updated_at`
	return DB.QueryRow(
		query,
		a.Title, a.Description, a.CreatedBy, a.Deadline,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// ListAssignments returns all assignments.
func ListAssignments() ([]Assignment, error) {
	var list []Assignment
	err := DB.Select(&list, `
    SELECT id, title, description, created_by, deadline, created_at, updated_at
    FROM assignments
    ORDER BY created_at DESC
  `)
	return list, err
}

// GetAssignment looks up one assignment by ID.
func GetAssignment(id int) (*Assignment, error) {
	var a Assignment
	err := DB.Get(&a, `
    SELECT id, title, description, created_by, deadline, created_at, updated_at
    FROM assignments
    WHERE id = $1
  `, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpdateAssignment modifies title/description/deadline of an existing assignment.
func UpdateAssignment(a *Assignment) error {
	res, err := DB.Exec(`
    UPDATE assignments
    SET title=$1, description=$2, deadline=$3, updated_at=now()
    WHERE id=$4
  `, a.Title, a.Description, a.Deadline, a.ID)
	if err != nil {
		return err
	}
	if cnt, _ := res.RowsAffected(); cnt == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

// DeleteAssignment removes an assignment (and cascades test_cases/submissions).
func DeleteAssignment(id int) error {
	_, err := DB.Exec(`DELETE FROM assignments WHERE id=$1`, id)
	return err
}
