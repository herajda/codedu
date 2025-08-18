CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  name TEXT,
  avatar TEXT,
  role TEXT NOT NULL DEFAULT 'student' CHECK (role IN ('student','teacher','admin')),
  theme TEXT NOT NULL DEFAULT 'light' CHECK (theme IN ('light','dark')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bk_class TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bk_uid TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS theme TEXT NOT NULL DEFAULT 'light' CHECK (theme IN ('light','dark'));


CREATE TABLE IF NOT EXISTS classes (
  id SERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  teacher_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS assignments (
  id SERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  created_by INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  deadline TIMESTAMPTZ NOT NULL,
  max_points INTEGER NOT NULL DEFAULT 100,
  grading_policy TEXT NOT NULL DEFAULT 'all_or_nothing' CHECK (grading_policy IN ('all_or_nothing','weighted')),
  published BOOLEAN NOT NULL DEFAULT FALSE,
  show_traceback BOOLEAN NOT NULL DEFAULT FALSE,
  template_path TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE
);

ALTER TABLE assignments ADD COLUMN IF NOT EXISTS template_path TEXT;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS show_traceback BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS test_cases (
  id SERIAL PRIMARY KEY,
  assignment_id INTEGER NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  stdin TEXT NOT NULL,
  expected_stdout TEXT NOT NULL,
  weight NUMERIC NOT NULL DEFAULT 1 CHECK (weight > 0),
  time_limit_sec NUMERIC NOT NULL DEFAULT 1.0,
  memory_limit_kb INTEGER NOT NULL DEFAULT 65536,
  unittest_code TEXT,
  unittest_name TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS unittest_code TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS unittest_name TEXT;

DO $$ BEGIN
    CREATE TYPE submission_status AS ENUM ('pending','running','completed','failed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS submissions (
  id SERIAL PRIMARY KEY,
  assignment_id INTEGER NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  student_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_path TEXT NOT NULL,
  code_content TEXT,
  status submission_status NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS points NUMERIC;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS override_points NUMERIC;

DO $$ BEGIN
    CREATE TYPE result_status AS ENUM ('passed','time_limit_exceeded','memory_limit_exceeded','wrong_output','runtime_error');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS results (
  id SERIAL PRIMARY KEY,
  submission_id INTEGER NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  test_case_id INTEGER NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
  status result_status NOT NULL,
  actual_stdout TEXT,
  stderr TEXT,
  exit_code INTEGER,
  runtime_ms INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE results ADD COLUMN IF NOT EXISTS stderr TEXT;
ALTER TABLE results ADD COLUMN IF NOT EXISTS exit_code INTEGER;

CREATE TABLE IF NOT EXISTS class_students (
  class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  student_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (class_id, student_id)
);

CREATE TABLE IF NOT EXISTS class_files (
  id SERIAL PRIMARY KEY,
  class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  parent_id INTEGER REFERENCES class_files(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  is_dir BOOLEAN NOT NULL DEFAULT FALSE,
  content BYTEA,
  size INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(class_id, path)
);
CREATE INDEX IF NOT EXISTS idx_class_files_path ON class_files(class_id, path);

-- Messages table for private chats
CREATE TABLE IF NOT EXISTS messages (
  id SERIAL PRIMARY KEY,
  sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  image TEXT,
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient_created
  ON messages(sender_id, recipient_id, created_at);

-- add image column if upgrading from an older schema
ALTER TABLE messages ADD COLUMN IF NOT EXISTS image TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS is_read BOOLEAN NOT NULL DEFAULT FALSE;

-- Blocked users table
CREATE TABLE IF NOT EXISTS blocked_users (
  blocker_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (blocker_id, blocked_id)
);

-- Starred conversations table
CREATE TABLE IF NOT EXISTS starred_conversations (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  other_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, other_id)
);

-- Archived conversations table
CREATE TABLE IF NOT EXISTS archived_conversations (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  other_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, other_id)
);

-- Class forum messages table
CREATE TABLE IF NOT EXISTS forum_messages (
  id SERIAL PRIMARY KEY,
  class_id INTEGER NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  image TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_forum_messages_class_created
  ON forum_messages(class_id, created_at);
ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS image TEXT;

