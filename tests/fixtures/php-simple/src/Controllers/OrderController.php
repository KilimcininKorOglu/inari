<?php

namespace App\Controllers;

use App\Payments\PaymentService;
use App\Payments\PaymentServiceInterface;
use App\Utils\Logger;

/** Handles order operations including payment processing. */
class OrderController
{
    private PaymentServiceInterface $paymentService;
    private Logger $logger;

    public function __construct(PaymentServiceInterface $paymentService, Logger $logger)
    {
        $this->paymentService = $paymentService;
        $this->logger = $logger;
    }

    public function createOrder(string $orderId, float $amount): bool
    {
        $this->logger->info("Creating order: {$orderId}");
        $result = $this->paymentService->processPayment($orderId, $amount);
        if ($result) {
            $this->logger->info("Order created successfully");
        }
        return $result;
    }

    public function cancelOrder(string $orderId): void
    {
        $this->logger->info("Cancelling order: {$orderId}");
        $this->paymentService->refundPayment($orderId);
    }

    public static function create(): self
    {
        $logger = new Logger("OrderController");
        $service = new PaymentService($logger);
        return new self($service, $logger);
    }
}
