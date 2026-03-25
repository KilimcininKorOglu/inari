import Foundation

/// Simple logging utility.
class Logger {
    private let context: String

    init(context: String) {
        self.context = context
    }

    func info(_ message: String) {
        print("[INFO] \(context): \(message)")
    }

    func error(_ message: String) {
        print("[ERROR] \(context): \(message)")
    }

    func debug(_ message: String) {
        print("[DEBUG] \(context): \(message)")
    }
}
