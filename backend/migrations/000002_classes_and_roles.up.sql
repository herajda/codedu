-- 000002_classes_and_roles.up.sql
------------------------------------------------
-- 1) Teachers created by admins

------------------------------------------------
-- 2) Classes (one teacher, many students)
CREATE TABLE classes (
  id            SERIAL PRIMARY KEY,
  name          TEXT NOT NULL,
  teacher_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3) Bridge table: class  ←→  student
CREATE TABLE class_students (
  class_id    INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  student_id  INTEGER NOT NULL REFERENCES users(id)   ON DELETE CASCADE,
  PRIMARY KEY (class_id, student_id)
);

------------------------------------------------
-- 4) Assignments now belong to a class (not system-wide)
ALTER TABLE assignments
  ADD COLUMN class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE;

