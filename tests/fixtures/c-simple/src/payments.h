#ifndef PAYMENTS_H
#define PAYMENTS_H

/* Payment result structure. */
typedef struct {
    int success;
    char message[256];
} PaymentResult;

/* Payment service structure. */
struct PaymentService {
    int retry_count;
};

int process_payment(struct PaymentService* service, const char* order_id, int amount);
void refund_payment(struct PaymentService* service, const char* transaction_id);

#endif
