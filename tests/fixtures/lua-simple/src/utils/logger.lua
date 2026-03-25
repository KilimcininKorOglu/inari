-- Simple logging utility.
local Logger = {}
Logger.__index = Logger

function Logger.new(context)
    local self = setmetatable({}, Logger)
    self.context = context
    return self
end

function Logger:info(message)
    print("[INFO] " .. self.context .. ": " .. message)
end

function Logger:error(message)
    io.stderr:write("[ERROR] " .. self.context .. ": " .. message .. "\n")
end

function Logger:debug(message)
    print("[DEBUG] " .. self.context .. ": " .. message)
end

return Logger
