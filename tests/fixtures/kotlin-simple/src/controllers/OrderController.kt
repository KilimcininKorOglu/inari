package controllers

import payments.PaymentService
import payments.IPaymentService
import utils.Logger

/** Handles order operations including payment processing. */
class OrderController(
    private val paymentService: IPaymentService,
    private val logger: Logger
) {
    fun createOrder(orderId: String, amount: Double): Boolean {
        logger.info("Creating order: $orderId")
        val result = paymentService.processPayment(orderId, amount)
        if (result) {
            logger.info("Order created successfully")
        }
        return result
    }

    fun cancelOrder(orderId: String) {
        logger.info("Cancelling order: $orderId")
        paymentService.refundPayment(orderId)
    }
}
