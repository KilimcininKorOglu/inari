package utils;

/** Simple logging utility. */
public class Logger {
    private final String context;

    public Logger(String context) {
        this.context = context;
    }

    public void info(String message) {
        System.out.println("[INFO] " + context + ": " + message);
    }

    public void error(String message) {
        System.err.println("[ERROR] " + context + ": " + message);
    }

    public void debug(String message) {
        System.out.println("[DEBUG] " + context + ": " + message);
    }
}
