package payments

/** Payment processing contract. */
interface IPaymentService {
    fun processPayment(orderId: String, amount: Double): Boolean
    fun refundPayment(transactionId: String)
}
