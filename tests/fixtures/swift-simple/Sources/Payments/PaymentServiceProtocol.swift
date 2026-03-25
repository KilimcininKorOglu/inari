/// Payment processing contract.
protocol PaymentServiceProtocol {
    func processPayment(orderId: String, amount: Double) -> Bool
    func refundPayment(transactionId: String)
}
