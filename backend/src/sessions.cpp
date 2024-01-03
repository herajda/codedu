#include "sessions.h"

Session::Session(std::shared_ptr<std::string> session_hash, const int user_id, uint valid_time_in_seconds) {
  updated_at = std::chrono::system_clock::now();
  this->session_hash = *session_hash;
  this->user_id = user_id;
  valid_time_in_seconds_from_updated_time = std::chrono::seconds(valid_time_in_seconds);
} 

bool Session::is_valid() const {
  return (updated_at + valid_time_in_seconds_from_updated_time) >= std::chrono::system_clock::now();
}
std::string Session::get_hash() const {
  return session_hash;
}

int Session::get_user_id() const {
  return user_id;
}

void Session::update() {
  updated_at = std::chrono::system_clock::now();
}


std::shared_ptr<std::string> Sessions::create_session(std::shared_ptr<std::string> user_name, std::shared_ptr<std::string> password, PostgresConnection psc) {
  if (user_name == nullptr || password == nullptr) {
    return std::make_shared<std::string>("");
  }
  std::string hashed_password = hash::hash_password(*password);
  // try to log in
  if (psc.login_user(std::make_shared<std::string>(user_name), std::make_shared<std::string>(hashed_password))) {
    // create session
    std::string session_hash = hash::gen_random_hash();
    Session session(session_hash, psc.get_user_id(user_name), 3600);
    std::unique_lock<std::shared_mutex> lock(sessions_mutex);
    sessions.insert(session);
    return std::make_shared<std::string>(session_hash);
  }
  else {
    return std::make_shared<std::string>("");
  }
}

bool Sessions::validate_session(const std::shared_ptr<std::string> session_hash) {
  std::shared_lock<std::shared_mutex> lock(sessions_mutex);
  auto session = sessions.find(*session_hash);
  if (session != sessions.end()) {
     if (session->second.is_valid()) {
       session->second.update();
       return true;
     }
     else {
       sessions.erase(session);
       return false;
     }
  }
  else {
    return false;
  }
}

int Sessions::get_user_id(const std::shared_ptr<std::string> session_hash) const {
  std::shared_lock<std::shared_mutex> lock(sessions_mutex);
  auto session = sessions.find(Session(*session_hash));
  if (session != sessions.end()) {
    return session.get_user_id();
  }
  else {
    return -1;
  }
}

