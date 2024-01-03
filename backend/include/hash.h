#pragma once

#include <string>
#include <sstream>
#include <iomanip>
#include <sodium.h>
#include <iostream>

class hash {
  public:
    static std::string hash_password(const std::string password, const std::string salt = get_salt());
    static bool verify_password(const std::string password, const std::string hash, const std::string salt = get_salt());
    static std::string get_salt();
    static std::string gen_random_hash();

  private:
    static std::string argon2(const std::string password);
    
    inline const static std::string SALT = "ffcKvu2WUZCGUPncSJnm9fFKtGMKeVvuKmrh8naB4zvo9yVAtt78v2ZnUyrMYJfZcyKGhWvoTVaYtkszbVZ8G2bqThtuF9FARv5QP2VkXMtbfE6DnsfoHDkuAogSYdbx";

    // salt for the hash_password function

};



