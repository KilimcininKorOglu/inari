#!/bin/bash
# Handles order operations including payment processing.

source ./lib/payments.sh
source ./lib/utils.sh

create_order() {
    local order_id="$1"
    local amount="$2"
    log_info "Creating order: $order_id"
    process_payment "$order_id" "$amount"
    local result=$?
    if [ $result -eq 0 ]; then
        log_info "Order created successfully"
    fi
    return $result
}

cancel_order() {
    local order_id="$1"
    log_info "Cancelling order: $order_id"
    refund_payment "$order_id"
}
