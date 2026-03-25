#ifndef UTILS_H
#define UTILS_H

#include <string>

namespace utils {

/// Simple logging utility.
class Logger {
public:
    Logger(const std::string& context);
    void info(const std::string& message);
    void error(const std::string& message);
    void debug(const std::string& message);

private:
    std::string context_;
};

} // namespace utils

#endif
