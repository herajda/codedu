package main

import (
    "log"
    "github.com/google/uuid"
)

// TeacherGroupID is a special class ID that represents the global Teachers' group.
// All teachers (and admins) may access its forum and files.
var TeacherGroupID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// EnsureTeacherGroupExists inserts the special Teachers group class row if missing.
// It assigns the group's teacher_id to an admin user to satisfy the FK; membership
// checks elsewhere grant access to all teachers/admins for this ID.
func EnsureTeacherGroupExists() {
    // Find any admin user to satisfy FK constraint
    var adminID uuid.UUID
    if err := DB.Get(&adminID, `SELECT id FROM users WHERE role='admin' ORDER BY created_at LIMIT 1`); err != nil {
        log.Printf("EnsureTeacherGroupExists: could not find admin user: %v", err)
        return
    }
    // Create class row if not exists
    if _, err := DB.Exec(`INSERT INTO classes (id, name, teacher_id) VALUES ($1,$2,$3) ON CONFLICT (id) DO NOTHING`, TeacherGroupID, "Teachers", adminID); err != nil {
        log.Printf("EnsureTeacherGroupExists: insert failed: %v", err)
    }
}

