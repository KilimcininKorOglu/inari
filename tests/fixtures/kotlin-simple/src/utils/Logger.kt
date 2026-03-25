package utils

/** Simple logging utility. */
class Logger(private val context: String) {
    fun info(message: String) {
        println("[INFO] $context: $message")
    }

    fun error(message: String) {
        System.err.println("[ERROR] $context: $message")
    }

    fun debug(message: String) {
        println("[DEBUG] $context: $message")
    }
}
