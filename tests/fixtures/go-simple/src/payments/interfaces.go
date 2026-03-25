package payments

// IPaymentService defines the payment service contract.
type IPaymentService interface {
	ProcessPayment(amount float64, userID string) (bool, error)
	RefundPayment(txID string) (bool, error)
}
