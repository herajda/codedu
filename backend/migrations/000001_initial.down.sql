-- migrations/000001_initial.down.sql

DROP TABLE IF EXISTS results;
DROP TYPE IF EXISTS result_status;

DROP TABLE IF EXISTS submissions;
DROP TYPE IF EXISTS submission_status;

DROP TABLE IF EXISTS test_cases;
DROP TABLE IF EXISTS assignments;
DROP TABLE IF EXISTS users;
