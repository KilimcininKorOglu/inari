#include "payments.h"
#include "utils.h"

/* Creates an order and processes payment. */
int create_order(struct PaymentService* service, const char* order_id, int amount) {
    log_info("Creating order");
    int result = process_payment(service, order_id, amount);
    if (result) {
        log_info("Order created successfully");
    }
    return result;
}

/* Cancels an order and refunds payment. */
void cancel_order(struct PaymentService* service, const char* order_id) {
    log_info("Cancelling order");
    refund_payment(service, order_id);
}
