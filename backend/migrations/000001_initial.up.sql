-- migrations/000001_initial.up.sql

-- 1) Users
CREATE TABLE users (
  id              SERIAL PRIMARY KEY,
  email           TEXT NOT NULL UNIQUE,
  password_hash   TEXT NOT NULL,
  role            TEXT NOT NULL DEFAULT 'student'
                    CHECK (role IN ('student','teacher','admin')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2) Assignments
CREATE TABLE assignments (
  id              SERIAL PRIMARY KEY,
  title           TEXT NOT NULL,
  description     TEXT NOT NULL,
  created_by      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  deadline        TIMESTAMPTZ NOT NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3) Test cases
CREATE TABLE test_cases (
  id               SERIAL PRIMARY KEY,
  assignment_id    INTEGER NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  stdin            TEXT NOT NULL,
  expected_stdout  TEXT NOT NULL,
  weight           NUMERIC NOT NULL DEFAULT 1 CHECK (weight > 0),
  time_limit_ms    INTEGER NOT NULL DEFAULT 1000,
  memory_limit_kb  INTEGER NOT NULL DEFAULT 65536,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4) Submission status enum + submissions table
CREATE TYPE submission_status AS ENUM ('pending','running','completed','failed');
CREATE TABLE submissions (
  id              SERIAL PRIMARY KEY,
  assignment_id   INTEGER NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  student_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_path       TEXT NOT NULL,
  status          submission_status NOT NULL DEFAULT 'pending',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 5) Result status enum + results table
CREATE TYPE result_status AS ENUM (
  'passed',
  'time_limit_exceeded',
  'memory_limit_exceeded',
  'wrong_output'
);
CREATE TABLE results (
  id               SERIAL PRIMARY KEY,
  submission_id    INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  test_case_id     INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
  status           result_status NOT NULL,
  actual_stdout    TEXT,
  runtime_ms       INTEGER,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

