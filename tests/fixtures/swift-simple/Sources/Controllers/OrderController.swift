/// Handles order operations including payment processing.
class OrderController {
    private let paymentService: PaymentServiceProtocol
    private let logger: Logger

    init(paymentService: PaymentServiceProtocol, logger: Logger) {
        self.paymentService = paymentService
        self.logger = logger
    }

    func createOrder(orderId: String, amount: Double) -> Bool {
        logger.info("Creating order: \(orderId)")
        let result = paymentService.processPayment(orderId: orderId, amount: amount)
        if result {
            logger.info("Order created successfully")
        }
        return result
    }

    func cancelOrder(orderId: String) {
        logger.info("Cancelling order: \(orderId)")
        paymentService.refundPayment(transactionId: orderId)
    }

    static func create() -> OrderController {
        let logger = Logger(context: "OrderController")
        let service = PaymentService(logger: logger)
        return OrderController(paymentService: service, logger: logger)
    }
}
