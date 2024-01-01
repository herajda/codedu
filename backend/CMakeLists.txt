cmake_minimum_required(VERSION 3.10)
project(server)

# Set the C++ standard to use
set(CMAKE_CXX_STANDARD 23)
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

#target_link_libraries(${PROJECT_NAME} PRIVATE pqxx)
# Assuming Crow is a header-only library and included in the include/crow_all.h
# If Crow needs to be linked as a library, use target_link_libraries()

# If you have other libraries to link, you can use target_link_libraries here.
# For example, if you're using Boost with Crow:
# find_package(Boost 1.71 REQUIRED)
# target_link_libraries(YourExecutableName Boost::boost)

