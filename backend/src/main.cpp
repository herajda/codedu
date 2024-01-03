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


  CROW_ROUTE(app, "/upload").methods("POST"_method)
    ([](const crow::request& req) {
     auto x = crow::json::load(req.body);
     if (!x)
     return crow::response(400, "Invalid request");

     std::string encoded_string = x["fileData"].s();
     std::string file_name = x["fileName"].s();
     std::string decoded_data = base64_decode(encoded_string.substr(encoded_string.find(",") + 1));

     std::ofstream out("uploaded_files/" + file_name, std::ios::binary);
     if (out) {
     out << decoded_data;
     out.close();
     return crow::response(200, "File uploaded successfully");
     } else {
     return crow::response(500, "Server error: Unable to save file");
     }
     });

  CROW_ROUTE(app, "/register").methods("POST"_method)
    ([](const crow::request& req) {
     auto x = crow::json::load(req.body);
     if (!x)
     return crow::response(400, "Invalid request");

     std::string username = x["username"].s();
     std::string password = x["password"].s();

     // Hash password
     std::string hashed_password = hash::hash_password(password); // Implement this function

     // Connect to PostgreSQL and insert new user
     std::cout << "Connecting to PostgreSQL database" << std::endl;
     try {

       std::string db_name = std::getenv("DB_NAME");
       std::string db_user = std::getenv("DB_USER");
       std::string db_password = std::getenv("DB_PASSWORD");
       std::string db_host = std::getenv("DB_HOST");
     
       std::string connection_str = "dbname = " + db_name + 
                                    " user = " + db_user +
                                    " password = " + db_password +
                                    " host = " + db_host;
       pqxx::connection c(connection_str);
       pqxx::work w(c);
       w.exec0("INSERT INTO users (username, password) VALUES (" + w.quote(username) + ", " + w.quote(hashed_password) + ")");
       w.commit();
     return crow::response(200, "User registered successfully");
     } catch (const std::exception &e) {
     std::cout << e.what() << std::endl;
     return crow::response(500, "Server error: Unable to register user");

     }
    });

  auto a = app.port(18080).multithreaded().run_async();
}

// Define the base64_decode function...
