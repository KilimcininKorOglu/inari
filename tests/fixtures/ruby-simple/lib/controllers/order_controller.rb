# Handles order operations including payment processing.
require_relative "../payments/payment_service"
require_relative "../utils/logger"

module Controllers
  class OrderController
    def initialize(payment_service, logger)
      @payment_service = payment_service
      @logger = logger
    end

    def create_order(order_id, amount)
      @logger.info("Creating order: #{order_id}")
      result = @payment_service.process_payment(order_id, amount)
      @logger.info("Order created successfully") if result
      result
    end

    def cancel_order(order_id)
      @logger.info("Cancelling order: #{order_id}")
      @payment_service.refund_payment(order_id)
    end
  end
end
