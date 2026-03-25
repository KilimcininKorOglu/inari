#ifndef PAYMENTS_H
#define PAYMENTS_H

#include <string>
#include "utils.h"

namespace payments {

/// Payment processing contract.
class PaymentInterface {
public:
    virtual bool processPayment(const std::string& orderId, double amount) = 0;
    virtual void refundPayment(const std::string& transactionId) = 0;
    virtual ~PaymentInterface() = default;
};

/// Handles payment processing with logging and validation.
class PaymentService : public PaymentInterface {
public:
    PaymentService(utils::Logger* logger);
    bool processPayment(const std::string& orderId, double amount) override;
    void refundPayment(const std::string& transactionId) override;
    static PaymentService* createDefault();

private:
    utils::Logger* logger_;
    int retryCount_;
    bool executeTransaction(const std::string& orderId, double amount);
};

} // namespace payments

#endif
