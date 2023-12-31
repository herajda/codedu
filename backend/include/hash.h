#pragma once

#include <string>
#include <openssl/sha.h>
#include <sstream>
#include <iomanip>


std::string sha256(const std::string str);
std::string hash_password(const std::string password);


