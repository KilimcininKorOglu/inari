package payments;

/** Payment processing contract. */
public interface IPaymentService {
    boolean processPayment(String orderId, double amount);
    void refundPayment(String transactionId);
}
