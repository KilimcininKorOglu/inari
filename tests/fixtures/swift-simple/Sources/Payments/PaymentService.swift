/// Handles payment processing with logging and validation.
class PaymentService: PaymentServiceProtocol {
    private let logger: Logger
    var retryCount: Int = 3

    init(logger: Logger) {
        self.logger = logger
    }

    func processPayment(orderId: String, amount: Double) -> Bool {
        logger.info("Processing payment for order: \(orderId)")
        if amount <= 0 {
            logger.error("Invalid amount: \(amount)")
            return false
        }
        return executeTransaction(orderId: orderId, amount: amount)
    }

    func refundPayment(transactionId: String) {
        logger.info("Refunding transaction: \(transactionId)")
    }

    static func createDefault() -> PaymentService {
        return PaymentService(logger: Logger(context: "PaymentService"))
    }

    private func executeTransaction(orderId: String, amount: Double) -> Bool {
        logger.info("Executing transaction for \(orderId)")
        return true
    }
}
