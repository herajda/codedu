#include "postgres_connection.h"

// Constructor
PostgresConnection::PostgresConnection(SharedString host, SharedString user, SharedString password, SharedString dbname) {}

// Setters
bool PostgresConnection::register_user(SharedString username, SharedString password_hash, SharedString first_name, SharedString last_name) const {
    return false;
}

bool PostgresConnection::login_user(SharedString username, SharedString password_hash) const {
    return false;
}

bool PostgresConnection::add_user_to_students(int user_id) const {
    return false;
}

bool PostgresConnection::add_user_to_teachers(int user_id) const {
    return false;
}

bool PostgresConnection::add_user_to_admins(int user_id) const {
    return false;
}

bool PostgresConnection::add_class(SharedString class_code, SharedString class_name, int teacher_user_id) const {
    return false;
}

bool PostgresConnection::add_student_to_class(int student_user_id, int class_id) const {
    return false;
}

bool PostgresConnection::add_teacher_to_class(int teacher_user_id, int class_id, bool owner) const {
    return false;
}

bool PostgresConnection::add_assignment(int class_id, SharedString assignment_name, SharedString assignment_description, SharedString due_timestamp, bool success_all_tests_correct) const {
    return false;
}

bool PostgresConnection::add_test_to_assignment(int class_id, int assignment_id, int test_points, SharedString test_name, uint test_number, SharedString test_stdin, SharedString expected_stdout, uint max_runtime, uint max_memory) const {
    return false;
}

bool PostgresConnection::add_submission(int assignment_id, int user_id, SharedString submission_timestamp, SharedString submission_comment, SharedString files_dir_path) const {
    return false;
}

bool PostgresConnection::add_evaluation_to_test_submission(int test_id, int submission_id, bool test_passed, uint exit_code, SharedString actual_stdout, SharedString actual_stderr, bool runtime_error, bool memory_error) const {
    return false;
}

bool PostgresConnection::remove_student_from_class(int student_user_id, int class_id) const {
    return false;
}

bool PostgresConnection::remove_teacher_from_class(int teacher_user_id, int class_id) const {
    return false;
}

bool PostgresConnection::remove_class(int class_id) const {
    return false;
}

bool PostgresConnection::remove_assignment(int assignment_id) const {
    return false;
}

bool PostgresConnection::edit_assignment_name(int assignment_id, SharedString assignment_name) const {
    return false;
}

bool PostgresConnection::edit_assignment_description(int assignment_id, SharedString assignment_description) const {
    return false;
}

bool PostgresConnection::edit_assignment_due(int assignment_id, SharedString due_timestamp) const {
    return false;
}

bool PostgresConnection::edit_assignment_success(int assignment_id, bool success_all_tests_correct) const {
    return false;
}

bool PostgresConnection::edit_test_name(int test_id, SharedString test_name) const {
    return false;
}

bool PostgresConnection::edit_test_number(int test_id, uint test_number) const {
    return false;
}

bool PostgresConnection::edit_test_stdin(int test_id, SharedString test_stdin) const {
    return false;
}

bool PostgresConnection::edit_test_stdout(int test_id, SharedString expected_stdout) const {
    return false;
}

bool PostgresConnection::edit_test_runtime(int test_id, uint max_runtime) const {
    return false;
}

bool PostgresConnection::edit_test_memory(int test_id, uint max_memory) const {
    return false;
}

bool PostgresConnection::edit_user_firsname(int user_id, SharedString first_name) const {
    return false;
}

bool PostgresConnection::edit_user_lastname(int user_id, SharedString last_name) const {
    return false;
}

// Getters
int PostgresConnection::get_user_id(SharedString username) const {
    return 0;
}

int PostgresConnection::get_class_id(SharedString class_code) const {
    return 0;
}

int PostgresConnection::get_assignment_id(SharedString assignment_name, int class_id) const {
    return 0;
}

int PostgresConnection::get_test_id(SharedString test_name, int assignment_id) const {
    return 0;
}

VectorInt PostgresConnection::get_submissions_ids(int assignment_id, int user_id) const {
    return {};
}

VectorInt PostgresConnection::get_tests_ids(int assignment_id) const {
    return {};
}

VectorInt PostgresConnection::get_classes_ids(int user_id) const {
    return {};
}

VectorInt PostgresConnection::get_assignments_ids(int class_id) const {
    return {};
}

VectorInt PostgresConnection::get_students_ids(int class_id) const {
    return {};
}

VectorInt PostgresConnection::get_teachers_ids(int class_id) const {
    return {};
}

VectorInt PostgresConnection::get_admins_ids() const {
    return {};
}

bool PostgresConnection::is_user_student(int user_id) const {
    return false;
}

bool PostgresConnection::is_user_teacher(int user_id) const {
    return false;
}

bool PostgresConnection::is_user_admin(int user_id) const {
    return false;
}

bool PostgresConnection::is_user_student_in_class(int user_id, int class_id) const {
    return false;
}

VectorInt PostgresConnection::get_student_ids_in_class(SharedString class_code, SharedString class_name) const {
    return {};
}

bool PostgresConnection::are_tests_finished(int submission_id) const {
    return false;
}

VariantMap PostgresConnection::get_submission_results(int submission_id) const {
    return {};
}

int PostgresConnection::get_points_for_assignment(int assignment_id, int user_id) const {
    return 0;
}

int PostgresConnection::get_points_for_test(int test_id, int submission_id) const {
    return 0;
}

VariantMap PostgresConnection::get_assignment_details(int assignment_id) const {
    return {};
}

VariantMap PostgresConnection::get_class_details(int class_id) const {
    return {};
}

VariantMap PostgresConnection::get_submission_details(int submission_id) const {
    return {};
}

VariantMap PostgresConnection::get_test_details(int test_id) const {
    return {};
}

VariantMap PostgresConnection::get_user_details(int user_id) const {
    return {};
}
