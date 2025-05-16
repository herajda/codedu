package main

import "time"

type User struct {
	ID           int       `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	CreatedAt    time.Time `db:"created_at"`
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
