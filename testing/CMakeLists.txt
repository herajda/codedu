cmake_minimum_required(VERSION 3.0)
project(server)
set(CMAKE_CXX_STANDARD 20)
set(SOURCES
    server.cpp
)
add_executable(server ${SOURCES})
target_include_directories(server PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})

