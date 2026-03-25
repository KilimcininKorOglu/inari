package controllers

import "example.com/go-simple/src/payments"

// OrderController handles order creation and management.
type OrderController struct {
	paymentService *payments.PaymentService
}

// NewOrderController creates a new OrderController.
func NewOrderController(ps *payments.PaymentService) *OrderController {
	return &OrderController{paymentService: ps}
}

// CreateOrder creates a new order and processes payment.
func (c *OrderController) CreateOrder(amount float64, userID string) error {
	ok, err := c.paymentService.ProcessPayment(amount, userID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return nil
}
