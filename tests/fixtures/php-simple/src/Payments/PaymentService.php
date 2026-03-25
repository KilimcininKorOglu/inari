<?php

namespace App\Payments;

use App\Utils\Logger;

/** Handles payment processing with logging and validation. */
class PaymentService implements PaymentServiceInterface
{
    private Logger $logger;
    protected int $retryCount = 3;

    public function __construct(Logger $logger)
    {
        $this->logger = $logger;
    }

    public function processPayment(string $orderId, float $amount): bool
    {
        $this->logger->info("Processing payment for order: {$orderId}");
        if ($amount <= 0) {
            $this->logger->error("Invalid amount: {$amount}");
            return false;
        }
        return $this->executeTransaction($orderId, $amount);
    }

    public function refundPayment(string $transactionId): void
    {
        $this->logger->info("Refunding transaction: {$transactionId}");
    }

    public static function createDefault(): self
    {
        return new self(new Logger("PaymentService"));
    }

    private function executeTransaction(string $orderId, float $amount): bool
    {
        $this->logger->info("Executing transaction for {$orderId}");
        return true;
    }
}
