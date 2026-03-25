#include "payments.h"

namespace payments {

PaymentService::PaymentService(utils::Logger* logger)
    : logger_(logger), retryCount_(3) {}

bool PaymentService::processPayment(const std::string& orderId, double amount) {
    logger_->info("Processing payment for order: " + orderId);
    if (amount <= 0) {
        logger_->error("Invalid amount");
        return false;
    }
    return executeTransaction(orderId, amount);
}

void PaymentService::refundPayment(const std::string& transactionId) {
    logger_->info("Refunding transaction: " + transactionId);
}

PaymentService* PaymentService::createDefault() {
    return new PaymentService(new utils::Logger("PaymentService"));
}

bool PaymentService::executeTransaction(const std::string& orderId, double amount) {
    logger_->info("Executing transaction for " + orderId);
    return true;
}

} // namespace payments
