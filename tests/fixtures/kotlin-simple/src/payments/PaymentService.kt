package payments

import utils.Logger

/** Handles payment processing with logging and validation. */
class PaymentService(private val logger: Logger) : IPaymentService {
    override fun processPayment(orderId: String, amount: Double): Boolean {
        logger.info("Processing payment for order: $orderId")
        if (amount <= 0) {
            logger.error("Invalid amount: $amount")
            return false
        }
        return executeTransaction(orderId, amount)
    }

    override fun refundPayment(transactionId: String) {
        logger.info("Refunding transaction: $transactionId")
    }

    private fun executeTransaction(orderId: String, amount: Double): Boolean {
        logger.info("Executing transaction for $orderId")
        return true
    }
}
