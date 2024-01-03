cmake_minimum_required(VERSION 3.14)
project(server)

# Set the C++ standard to use
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED True)

# Include directories
# Find all source files in the src directory
#file(GLOB SOURCES "src/*.cpp")
file(GLOB_RECURSE SOURCES "src/*.cpp")
include_directories(include)
#include_directories(${PostgreSQL_INCLUDE_DIRS})
# Add the executable
add_executable(server ${SOURCES})
find_package(OpenSSL REQUIRED)
if(OPENSSL_FOUND)
    target_include_directories(server PRIVATE ${OPENSSL_INCLUDE_DIR})
    target_link_libraries(server ${OPENSSL_LIBRARIES})
endif()

# Find and link PostgreSQL
find_package(PostgreSQL REQUIRED)
if(PostgreSQL_FOUND)
    include_directories(${PostgreSQL_INCLUDE_DIRS})
    target_link_libraries(server ${PostgreSQL_LIBRARIES})
endif()
target_link_libraries(server pqxx pq)

include(FetchContent)
FetchContent_Declare(Sodium
    GIT_REPOSITORY https://github.com/robinlinden/libsodium-cmake.git
    GIT_TAG 99f14233eab1d4f7f49c2af4ec836f2e701c445e # HEAD as of 2022-05-28
)
set(SODIUM_DISABLE_TESTS ON)
FetchContent_MakeAvailable(Sodium)

target_link_libraries(server sodium)

