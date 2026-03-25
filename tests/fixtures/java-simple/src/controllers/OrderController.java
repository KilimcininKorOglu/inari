package controllers;

import payments.PaymentService;
import payments.IPaymentService;
import utils.Logger;

/** Handles order operations including payment processing. */
public class OrderController {
    private final IPaymentService paymentService;
    private final Logger logger;

    public OrderController(IPaymentService paymentService, Logger logger) {
        this.paymentService = paymentService;
        this.logger = logger;
    }

    public boolean createOrder(String orderId, double amount) {
        logger.info("Creating order: " + orderId);
        boolean result = paymentService.processPayment(orderId, amount);
        if (result) {
            logger.info("Order created successfully");
        }
        return result;
    }

    public void cancelOrder(String orderId) {
        logger.info("Cancelling order: " + orderId);
        paymentService.refundPayment(orderId);
    }

    public static OrderController create() {
        Logger logger = new Logger("OrderController");
        PaymentService service = new PaymentService(logger);
        return new OrderController(service, logger);
    }
}
