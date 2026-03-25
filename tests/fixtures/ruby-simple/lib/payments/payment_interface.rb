# Payment processing contract.
module Payments
  module PaymentInterface
    def process_payment(order_id, amount)
      raise NotImplementedError
    end

    def refund_payment(transaction_id)
      raise NotImplementedError
    end
  end
end
