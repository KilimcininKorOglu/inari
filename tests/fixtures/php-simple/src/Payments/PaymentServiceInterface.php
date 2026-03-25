<?php

namespace App\Payments;

/** Payment processing contract. */
interface PaymentServiceInterface
{
    public function processPayment(string $orderId, float $amount): bool;
    public function refundPayment(string $transactionId): void;
}
