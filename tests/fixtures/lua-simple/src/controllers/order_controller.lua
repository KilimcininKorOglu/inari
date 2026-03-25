-- Handles order operations including payment processing.
local PaymentService = require("payments.payment_service")
local Logger = require("utils.logger")

local OrderController = {}
OrderController.__index = OrderController

function OrderController.new(paymentService, logger)
    local self = setmetatable({}, OrderController)
    self.paymentService = paymentService
    self.logger = logger
    return self
end

function OrderController:createOrder(orderId, amount)
    self.logger:info("Creating order: " .. orderId)
    local result = self.paymentService:processPayment(orderId, amount)
    if result then
        self.logger:info("Order created successfully")
    end
    return result
end

function OrderController:cancelOrder(orderId)
    self.logger:info("Cancelling order: " .. orderId)
    self.paymentService:refundPayment(orderId)
end

return OrderController
