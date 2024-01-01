#pragma once

#include <pqxx/pqxx>

class PostgresConnection {
  public:
    PostgresConnection(std::string host, std::string user, std::string password, std::string dbname);
    
    bool register_user(std::string username, std::string password_hash, std::string first_name, std::string last_name);
    bool login_user(std::string username, std::string password);
    



};
