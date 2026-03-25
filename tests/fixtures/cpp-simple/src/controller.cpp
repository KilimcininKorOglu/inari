#include "payments.h"
#include "utils.h"

namespace controllers {

/// Handles order operations including payment processing.
class OrderController {
public:
    OrderController(payments::PaymentInterface* paymentService, utils::Logger* logger)
        : paymentService_(paymentService), logger_(logger) {}

    bool createOrder(const std::string& orderId, double amount) {
        logger_->info("Creating order: " + orderId);
        bool result = paymentService_->processPayment(orderId, amount);
        if (result) {
            logger_->info("Order created successfully");
        }
        return result;
    }

    void cancelOrder(const std::string& orderId) {
        logger_->info("Cancelling order: " + orderId);
        paymentService_->refundPayment(orderId);
    }

private:
    payments::PaymentInterface* paymentService_;
    utils::Logger* logger_;
};

} // namespace controllers
