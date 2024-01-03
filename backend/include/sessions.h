#pragma once
#include <string>
#include <memory>
#include <chrono>
#include <shared_mutex>
#include <mutex>
#include <unordered_map>
#include <utility>
#include <tuple>

#include "hash.h"
#include "postgres_connection.h"


class Session 
{

  private:
    std::string session_hash;
    int user_id;
    std::chrono::time_point<std::chrono::system_clock> updated_at;
    std::chrono::seconds valid_time_in_seconds_from_updated_time;

  public:
    Session(std::shared_ptr<std::string> session_hash, const int user_id, uint valid_time_in_seconds);


    bool is_valid() const;

    std::string get_hash() const;

    int get_user_id() const;

    void update();

};

class Sessions {
  private:
    std::unordered_map<std::string, Session> sessions;
    mutable std::shared_mutex sessions_mutex;

  public:

    std::shared_ptr<std::string> create_session(std::shared_ptr<std::string> user_name, std::shared_ptr<std::string> password, PostgresConnection psc);

    bool validate_session(const std::shared_ptr<std::string> session_hash);
    void delete_all_sessions();
    int get_user_id(const std::shared_ptr<std::string> session_hash) const;

};
