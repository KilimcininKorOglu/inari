cmake_minimum_required(VERSION 3.10)
project(cpp-simple CXX)
set(CMAKE_CXX_STANDARD 17)
add_executable(main src/controller.cpp src/payments.cpp src/utils.cpp)
