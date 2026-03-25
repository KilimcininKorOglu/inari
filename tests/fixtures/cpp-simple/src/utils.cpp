#include "utils.h"
#include <iostream>

namespace utils {

Logger::Logger(const std::string& context) : context_(context) {}

void Logger::info(const std::string& message) {
    std::cout << "[INFO] " << context_ << ": " << message << std::endl;
}

void Logger::error(const std::string& message) {
    std::cerr << "[ERROR] " << context_ << ": " << message << std::endl;
}

void Logger::debug(const std::string& message) {
    std::cout << "[DEBUG] " << context_ << ": " << message << std::endl;
}

} // namespace utils
