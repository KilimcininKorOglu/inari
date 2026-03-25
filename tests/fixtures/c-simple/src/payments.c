#include "payments.h"
#include "utils.h"

/* Processes a payment for the given order. */
int process_payment(struct PaymentService* service, const char* order_id, int amount) {
    log_info("Processing payment");
    if (amount <= 0) {
        log_error("Invalid amount");
        return 0;
    }
    return execute_transaction(service, order_id, amount);
}

/* Refunds a payment transaction. */
void refund_payment(struct PaymentService* service, const char* transaction_id) {
    log_info("Refunding transaction");
}

static int execute_transaction(struct PaymentService* service, const char* order_id, int amount) {
    log_info("Executing transaction");
    return 1;
}
