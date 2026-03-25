#include <stdio.h>
#include "utils.h"

/* Logs an informational message. */
void log_info(const char* message) {
    printf("[INFO] %s\n", message);
}

/* Logs an error message. */
void log_error(const char* message) {
    fprintf(stderr, "[ERROR] %s\n", message);
}
