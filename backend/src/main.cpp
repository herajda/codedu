#include "crow_all.h"
#include <fstream>
#include "json.hpp"
#include "base64.h"
#include "hash.h"

#include <string>
#include <cstdlib>
#include <vector>
#include <pqxx/pqxx>


int main() {
  crow::SimpleApp app;

  CROW_ROUTE(app, "/register").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/login").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/teacher/login").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_user_to_students").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_user_to_teachers").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_user_to_admins").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_class").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_student_to_class").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_teacher_to_class").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_assignment").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_test_to_assignment").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_submission").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/add_evaluation_to_test_submission").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/remove_student_from_class").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/remove_teacher_from_class").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/remove_class").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/remove_assignment").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_assignment_name").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_assignment_description").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_assignment_due").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_assignment_success").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_test_name").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_test_number").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_test_stdin").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_test_stdout").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_test_runtime").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_test_memory").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_user_firstname").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/edit_user_lastname").methods("POST"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  // Continuing with the getters
  CROW_ROUTE(app, "/get_user_id").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_class_id").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_assignment_id").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_test_id").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_submissions_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_tests_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_classes_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_assignments_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_students_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_teachers_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_admins_ids").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/is_user_student").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/is_user_teacher").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/is_user_admin").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/is_user_student_in_class").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_student_ids_in_class").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/are_tests_finished").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_submission_results").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_points_for_assignment").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_points_for_test").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_assignment_details").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_class_details").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_submission_details").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_test_details").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  CROW_ROUTE(app, "/get_user_details").methods("GET"_method)
    ([](const crow::request& req) {
     return "Hello world";
     });

  auto a = app.port(18080).multithreaded().run_async();
}

// Define the base64_decode function...
