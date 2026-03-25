# Simple logging utility.
module Utils
  class Logger
    attr_reader :context

    def initialize(context)
      @context = context
    end

    def info(message)
      puts "[INFO] #{@context}: #{message}"
    end

    def error(message)
      $stderr.puts "[ERROR] #{@context}: #{message}"
    end

    def debug(message)
      puts "[DEBUG] #{@context}: #{message}"
    end
  end
end
