package payments;

import utils.Logger;

/** Handles payment processing with logging and validation. */
public class PaymentService implements IPaymentService {
    private final Logger logger;
    private static final double MAX_AMOUNT = 10000.0;

    public PaymentService(Logger logger) {
        this.logger = logger;
    }

    @Override
    public boolean processPayment(String orderId, double amount) {
        logger.info("Processing payment for order: " + orderId);
        if (amount <= 0 || amount > MAX_AMOUNT) {
            logger.error("Invalid amount: " + amount);
            return false;
        }
        return executeTransaction(orderId, amount);
    }

    @Override
    public void refundPayment(String transactionId) {
        logger.info("Refunding transaction: " + transactionId);
    }

    private boolean executeTransaction(String orderId, double amount) {
        logger.info("Executing transaction for " + orderId);
        return true;
    }

    public static PaymentService createDefault() {
        return new PaymentService(new Logger("PaymentService"));
    }
}
