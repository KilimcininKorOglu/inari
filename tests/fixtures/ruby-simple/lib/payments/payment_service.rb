# Handles payment processing with logging and validation.
require_relative "../utils/logger"
require_relative "payment_interface"

module Payments
  class PaymentService
    include PaymentInterface

    attr_reader :logger
    attr_accessor :retry_count

    def initialize(logger)
      @logger = logger
      @retry_count = 3
    end

    def process_payment(order_id, amount)
      logger.info("Processing payment for order: #{order_id}")
      if amount <= 0
        logger.error("Invalid amount: #{amount}")
        return false
      end
      execute_transaction(order_id, amount)
    end

    def refund_payment(transaction_id)
      logger.info("Refunding transaction: #{transaction_id}")
      true
    end

    def self.from_config(config)
      new(Utils::Logger.new(config[:context]))
    end

    private

    def execute_transaction(order_id, amount)
      logger.info("Executing transaction for #{order_id}")
      true
    end
  end
end
