package payments

import "example.com/go-simple/src/utils"

// PaymentService handles payment processing operations.
type PaymentService struct {
	logger *utils.Logger
}

// NewPaymentService creates a new PaymentService instance.
func NewPaymentService(logger *utils.Logger) *PaymentService {
	return &PaymentService{logger: logger}
}

// ProcessPayment processes a payment for the given amount and user.
func (s *PaymentService) ProcessPayment(amount float64, userID string) (bool, error) {
	s.logger.Info("Processing payment")
	if !s.validateAmount(amount) {
		return false, nil
	}
	return true, nil
}

// RefundPayment refunds a transaction.
func (s *PaymentService) RefundPayment(txID string) (bool, error) {
	s.logger.Info("Refunding payment")
	return true, nil
}

// validateAmount checks that the amount is positive.
func (s *PaymentService) validateAmount(amount float64) bool {
	return amount > 0
}
