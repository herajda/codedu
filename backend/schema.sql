-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  name TEXT,
  avatar TEXT,
  role TEXT NOT NULL DEFAULT 'student' CHECK (role IN ('student','teacher','admin')),
  theme TEXT NOT NULL DEFAULT 'light' CHECK (theme IN ('light','dark')),
  preferred_locale TEXT CHECK (preferred_locale IN ('en','cs')),
  email_notifications BOOLEAN NOT NULL DEFAULT TRUE,
  email_message_digest BOOLEAN NOT NULL DEFAULT TRUE,
  email_verified BOOLEAN NOT NULL DEFAULT TRUE,
  email_verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bk_class TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS bk_uid TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS theme TEXT NOT NULL DEFAULT 'light' CHECK (theme IN ('light','dark'));
ALTER TABLE users ADD COLUMN IF NOT EXISTS preferred_locale TEXT CHECK (preferred_locale IN ('en','cs'));
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_notifications BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_message_digest BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS ms_oid TEXT UNIQUE;

CREATE TABLE IF NOT EXISTS teacher_whitelist (
  email TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE IF NOT EXISTS classes (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name TEXT NOT NULL,
  teacher_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  description TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE classes ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
-- Remove legacy Teachers' group class (fixed ID) if present.
DELETE FROM classes WHERE id = '11111111-1111-1111-1111-111111111111';

CREATE TABLE IF NOT EXISTS assignments (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  deadline TIMESTAMPTZ NOT NULL,
  max_points INTEGER NOT NULL DEFAULT 100,
  max_submission_size_mb INTEGER NOT NULL DEFAULT 10 CHECK (max_submission_size_mb > 0),
  grading_policy TEXT NOT NULL DEFAULT 'all_or_nothing' CHECK (grading_policy IN ('all_or_nothing','weighted')),
  published BOOLEAN NOT NULL DEFAULT FALSE,
  show_traceback BOOLEAN NOT NULL DEFAULT FALSE,
  show_test_details BOOLEAN NOT NULL DEFAULT FALSE,
  programming_language TEXT NOT NULL DEFAULT 'python' CHECK (programming_language IN ('python','scratch')),
  manual_review BOOLEAN NOT NULL DEFAULT FALSE,
  banned_functions TEXT[] NOT NULL DEFAULT '{}',
  banned_modules TEXT[] NOT NULL DEFAULT '{}',
  banned_tool_rules TEXT,
  template_path TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE
);

ALTER TABLE assignments ADD COLUMN IF NOT EXISTS template_path TEXT;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS show_traceback BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS show_test_details BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS programming_language TEXT NOT NULL DEFAULT 'python' CHECK (programming_language IN ('python','scratch'));
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS manual_review BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS banned_functions TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS banned_modules TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS banned_tool_rules TEXT;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS max_submission_size_mb INTEGER NOT NULL DEFAULT 10 CHECK (max_submission_size_mb > 0);
-- LLM interactive testing configuration
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_interactive BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_feedback BOOLEAN NOT NULL DEFAULT FALSE; -- show LLM feedback to students
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_auto_award BOOLEAN NOT NULL DEFAULT TRUE; -- auto-award max points if all scenarios pass
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_scenarios_json TEXT; -- JSON describing interactive scenarios
-- Strictness slider (0-100) and teacher rubric for OK/Wrong definitions
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_strictness INTEGER NOT NULL DEFAULT 50; -- 0=Beginner (lenient), 100=Pro (strict)
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_rubric TEXT; -- freeform teacher guidance on what is OK vs WRONG
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_teacher_baseline_json TEXT; -- plan+results JSON from teacher standard solution (defines accepted behavior)
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS llm_help_why_failed BOOLEAN NOT NULL DEFAULT FALSE;


-- Second deadline feature
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS second_deadline TIMESTAMPTZ; -- optional second deadline for late submissions
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS late_penalty_ratio NUMERIC NOT NULL DEFAULT 0.5 CHECK (late_penalty_ratio >= 0 AND late_penalty_ratio <= 1); -- points multiplier for second deadline submissions
ALTER TABLE assignments ADD COLUMN IF NOT EXISTS scratch_semantic_criteria TEXT;

-- Track cloned assignments (e.g., Teachers' group versions)
CREATE TABLE IF NOT EXISTS assignment_clones (
  source_assignment_id UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  cloned_assignment_id UUID NOT NULL UNIQUE REFERENCES assignments(id) ON DELETE CASCADE,
  target_class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_assignment_clones_source ON assignment_clones(source_assignment_id);
CREATE INDEX IF NOT EXISTS idx_assignment_clones_target_class ON assignment_clones(target_class_id);

-- Per-student deadline override (extensions)
CREATE TABLE IF NOT EXISTS assignment_deadline_overrides (
  assignment_id UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  new_deadline TIMESTAMPTZ NOT NULL,
  note TEXT,
  created_by UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (assignment_id, student_id)
);
CREATE INDEX IF NOT EXISTS idx_ado_assignment ON assignment_deadline_overrides(assignment_id);
CREATE INDEX IF NOT EXISTS idx_ado_student ON assignment_deadline_overrides(student_id);

CREATE TABLE IF NOT EXISTS test_cases (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  assignment_id UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  stdin TEXT NOT NULL,
  expected_stdout TEXT NOT NULL,
  weight NUMERIC NOT NULL DEFAULT 1 CHECK (weight > 0),
  time_limit_sec NUMERIC NOT NULL DEFAULT 1.0,
  memory_limit_kb INTEGER NOT NULL DEFAULT 65536,
  unittest_code TEXT,
  unittest_name TEXT,
  execution_mode TEXT NOT NULL DEFAULT 'stdin_stdout',
  function_name TEXT,
  function_args TEXT,
  function_kwargs TEXT,
  function_arg_names TEXT,
  expected_return TEXT,
  file_name TEXT,
  file_base64 TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS unittest_code TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS unittest_name TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS execution_mode TEXT NOT NULL DEFAULT 'stdin_stdout';
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS function_name TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS function_args TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS function_kwargs TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS function_arg_names TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS expected_return TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS file_name TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS file_base64 TEXT;
ALTER TABLE test_cases ADD COLUMN IF NOT EXISTS files_json TEXT;

DO $$ BEGIN
    CREATE TYPE submission_status AS ENUM ('pending','running','completed','failed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS submissions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  assignment_id UUID NOT NULL REFERENCES assignments(id) ON DELETE CASCADE,
  student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_path TEXT NOT NULL,
  code_content TEXT,
  status submission_status NOT NULL DEFAULT 'pending',
  late BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE submissions ADD COLUMN IF NOT EXISTS points NUMERIC;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS override_points NUMERIC;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS is_teacher_run BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS late BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS manually_accepted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS all_tests_failure_explanation TEXT;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS scratch_analysis TEXT;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS scratch_semantic_analysis TEXT;

DO $$ BEGIN
    CREATE TYPE result_status AS ENUM ('passed','time_limit_exceeded','memory_limit_exceeded','wrong_output','runtime_error');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

ALTER TYPE result_status ADD VALUE IF NOT EXISTS 'illegal_tool_use';

CREATE TABLE IF NOT EXISTS results (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  test_case_id UUID NOT NULL REFERENCES test_cases(id) ON DELETE CASCADE,
  status result_status NOT NULL,
  actual_stdout TEXT,
  stderr TEXT,
  exit_code INTEGER,
  runtime_ms INTEGER,
  actual_return TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE results ADD COLUMN IF NOT EXISTS stderr TEXT;
ALTER TABLE results ADD COLUMN IF NOT EXISTS exit_code INTEGER;
ALTER TABLE results ADD COLUMN IF NOT EXISTS actual_return TEXT;
ALTER TABLE results ADD COLUMN IF NOT EXISTS failure_explanation TEXT;

-- LLM run artifacts per submission attempt
CREATE TABLE IF NOT EXISTS llm_runs (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  smoke_ok BOOLEAN NOT NULL DEFAULT FALSE,
  review_json TEXT,
  interactive_json TEXT,
  transcript TEXT,
  verdict TEXT,      -- PASS or failure category (SMOKE_FAIL, INTERACTIVE_TIMEOUT, OUTPUT_TRUNCATED, SCENARIO_FAIL, RUNTIME_ERROR, etc.)
  reason TEXT,       -- short human-readable explanation
  model_name TEXT,
  tool_calls INTEGER,
  wall_time_ms INTEGER,
  output_size INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS class_students (
  class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (class_id, student_id)
);

CREATE TABLE IF NOT EXISTS class_files (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  owner_id UUID REFERENCES users(id) ON DELETE CASCADE,
  parent_id UUID REFERENCES class_files(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  path TEXT NOT NULL,
  is_dir BOOLEAN NOT NULL DEFAULT FALSE,
  assignment_id UUID REFERENCES assignments(id) ON DELETE SET NULL,
  content BYTEA,
  size INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_class_files_path ON class_files(class_id, path);
-- Backfill column if upgrading
ALTER TABLE class_files ADD COLUMN IF NOT EXISTS assignment_id UUID REFERENCES assignments(id) ON DELETE SET NULL;
ALTER TABLE class_files ADD COLUMN IF NOT EXISTS owner_id UUID REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE class_files DROP CONSTRAINT IF EXISTS class_files_class_id_path_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_class_files_class_path_unique ON class_files(class_id, path) WHERE owner_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_class_files_owner_path_unique ON class_files(owner_id, path) WHERE owner_id IS NOT NULL;

-- Messages table for private chats
CREATE TABLE IF NOT EXISTS messages (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  image TEXT,
  is_read BOOLEAN NOT NULL DEFAULT FALSE,
  structured BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_messages_sender_recipient_created
  ON messages(sender_id, recipient_id, created_at);

-- add image column if upgrading from an older schema
ALTER TABLE messages ADD COLUMN IF NOT EXISTS image TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS is_read BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS structured BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS file_name TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS file TEXT;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS reply_to_id UUID REFERENCES messages(id) ON DELETE SET NULL;

-- Track completed email notifications to avoid duplicates
CREATE TABLE IF NOT EXISTS notification_log (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  notification_type TEXT NOT NULL,
  context TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(user_id, notification_type, context)
);

-- Blocked users table
CREATE TABLE IF NOT EXISTS blocked_users (
  blocker_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (blocker_id, blocked_id)
);

-- Starred conversations table
CREATE TABLE IF NOT EXISTS starred_conversations (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  other_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, other_id)
);

-- Archived conversations table
CREATE TABLE IF NOT EXISTS archived_conversations (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  other_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, other_id)
);

-- Class forum messages table
CREATE TABLE IF NOT EXISTS forum_messages (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  image TEXT,
  structured BOOLEAN NOT NULL DEFAULT FALSE,
  file_name TEXT,
  file TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_forum_messages_class_created
  ON forum_messages(class_id, created_at);
ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS image TEXT;
ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS structured BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS file_name TEXT;
ALTER TABLE forum_messages ADD COLUMN IF NOT EXISTS file TEXT;

-- User online status tracking
CREATE TABLE IF NOT EXISTS user_presence (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  is_online BOOLEAN NOT NULL DEFAULT FALSE,
  last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for efficient queries
CREATE INDEX IF NOT EXISTS idx_user_presence_online ON user_presence(is_online);
CREATE INDEX IF NOT EXISTS idx_user_presence_last_seen ON user_presence(last_seen);

-- Password reset tokens
CREATE TABLE IF NOT EXISTS password_reset_tokens (
  token_hash TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id);

-- Email verification tokens
CREATE TABLE IF NOT EXISTS email_verification_tokens (
  token_hash TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user ON email_verification_tokens(user_id);

-- System-wide configuration settings
CREATE TABLE IF NOT EXISTS system_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- Default settings
INSERT INTO system_settings (key, value) VALUES ('force_bakalari_email', 'true') ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS submission_test_explanations (
  submission_id UUID NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
  test_case_id TEXT NOT NULL,
  explanation TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (submission_id, test_case_id)
);
