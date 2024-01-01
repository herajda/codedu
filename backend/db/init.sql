CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL,
);

CREATE TABLE IF NOT EXISTS students (
    id SERIAL PRIMARY KEY, 
    user_id INTEGER UNIQUE,
    FOREIGN KEY (user_id) REFERENCES users(id)
) 

CREATE TABLE IF NOT EXISTS teachers (
    id SERIAL PRIMARY KEY, 
    user_id INTEGER UNIQUE,
    FOREIGN KEY (user_id) REFERENCES users(id)
)

CREATE TABLE IF NOT EXISTS classes (
    id SERIAL PRIMARY KEY, 
    class_name VARCHAR(255) NOT NULL,
    class_code VARCHAR(255) NOT NULL
)
CREATE TABLE IF NOT EXISTS class_students (
    id SERIAL PRIMARY KEY,
    class_id INT NOT NULL,
    student_id INT NOT NULL,
    FOREIGN KEY (class_id) REFERENCES classes (class_id),
    FOREIGN KEY (student_id) REFERENCES students (student_id),
    UNIQUE (class_id, student_id)
);

CREATE TABLE IF NOT EXISTS class_teachers (
    id SERIAL PRIMARY KEY,
    class_id INT NOT NULL,
    teacher_id INT NOT NULL,
    FOREIGN KEY (class_id) REFERENCES classes (class_id),
    FOREIGN KEY (teacher_id) REFERENCES teachers (teacher_id),
    UNIQUE (class_id, teacher_id)
);

CREATE TABLE IF NOT EXISTS assignments (
    id SERIAL PRIMARY KEY, 
    class_id INTEGER REFERENCES classes(id),
    assignment_name TEXT NOT NULL,
    assignment_description VARCHAR(255) NOT NULL,
    assignment_due_date DATE NOT NULL,
    success_all_tests_correct BOOLEAN NOT NULL DEFAULT false, -- if true, all tests must pass to be considered correct; defult false
);


CREATE TABLE IF NOT EXISTS submissions (
    id SERIAL PRIMARY KEY,
    assignment_id INT NOT NULL,
    student_id INT NOT NULL,
    submission_number INT NOT NULL,
    submission_date DATE NOT NULL,
    submission_comment TEXT,
    FOREIGN KEY (assignment_id) REFERENCES assignments (assignment_id),
    FOREIGN KEY (student_id) REFERENCES students (student_id),
    UNIQUE (assignment_id, student_id, submission_number)
);


CREATE TABLE IF NOT EXISTS files (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER REFERENCES submissions(id),
    file_data BYTEA NOT NULL,
    file_name VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS tests (
    id SERIAL PRIMARY KEY,
    assignment_id INTEGER REFERENCES assignments(id),
    test_name VARCHAR(255) NOT NULL,
    test_number INTEGER NOT NULL,
    stdin TEXT NOT NULL,
    expected_stdout TEXT NOT NULL,
    expected_stderr TEXT,
    max_runtime INTEGER,
    max_memory INTEGER,
    UNIQUE (assignment_id, test_number)
);

CREATE TABLE IF NOT EXISTS test_evaluations (
    id SERIAL PRIMARY KEY,
    test_id INTEGER UNIQUE REFERENCES tests(id),
    submission_id INTEGER REFERENCES submissions(id),
    BOOLEAN test_passed NOT NULL,
    exit_code INTEGER NOT NULL,
    test_output TEXT NOT NULL,
);

CREATE OR REPLACE FUNCTION auto_increment_submission_number()
RETURNS TRIGGER AS $$
BEGIN
    NEW.submission_number := (SELECT COALESCE(MAX(submission_number), 0) + 1
                              FROM submissions
                              WHERE assignment_id = NEW.assignment_id
                              AND student_id = NEW.student_id);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER increment_submission_number
BEFORE INSERT ON submissions
FOR EACH ROW EXECUTE FUNCTION auto_increment_submission_number();
