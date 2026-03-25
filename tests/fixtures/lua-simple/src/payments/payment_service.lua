-- Handles payment processing with logging and validation.
local Logger = require("utils.logger")

local PaymentService = {}
PaymentService.__index = PaymentService

function PaymentService.new(logger)
    local self = setmetatable({}, PaymentService)
    self.logger = logger
    self.retryCount = 3
    return self
end

function PaymentService:processPayment(orderId, amount)
    self.logger:info("Processing payment for order: " .. orderId)
    if amount <= 0 then
        self.logger:error("Invalid amount: " .. tostring(amount))
        return false
    end
    return self:executeTransaction(orderId, amount)
end

function PaymentService:refundPayment(transactionId)
    self.logger:info("Refunding transaction: " .. transactionId)
    return true
end

function PaymentService.createDefault()
    return PaymentService.new(Logger.new("PaymentService"))
end

local function executeTransaction(self, orderId, amount)
    self.logger:info("Executing transaction for " .. orderId)
    return true
end

PaymentService.executeTransaction = executeTransaction

return PaymentService
