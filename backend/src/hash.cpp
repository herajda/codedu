#include "hash.h"

std::string hash::hash_password(const std::string password, const std::string salt) {
    std::string salted_password = password + salt;
    return argon2(salted_password);
}

bool hash::verify_password(const std::string password, const std::string hash, const std::string salt) {
    std::string salted_password = password + salt;
    if (argon2(salted_password) == hash) {
        return true;
    }
    return false;
    
}

std::string hash::get_salt() {
    return SALT;
}

std::string hash::argon2(const std::string password) {
  char hash[crypto_pwhash_STRBYTES];
  if (crypto_pwhash_str(
    hash,
    password.c_str(),
    password.length(),
    crypto_pwhash_OPSLIMIT_INTERACTIVE,
    crypto_pwhash_MEMLIMIT_INTERACTIVE
  ) != 0) {
    throw std::runtime_error("Error hashing password");
  }
  std::cout << "Hashed password: " << hash << std::endl;
  return std::string(reinterpret_cast<char*>(hash));
}


std::string hash::gen_random_hash() {
  return "";
}
