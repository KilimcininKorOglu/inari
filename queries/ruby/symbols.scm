;; Class definitions
(class
  name: (constant) @name) @definition

;; Module definitions
(module
  name: (constant) @name) @definition

;; Instance method definitions
(method
  name: (identifier) @name) @definition

;; Singleton (class-level) method definitions (e.g. def self.from_config)
(singleton_method
  name: (identifier) @name) @definition

;; attr_accessor / attr_reader / attr_writer as property symbols
(call
  method: (identifier) @_attr_method
  arguments: (argument_list
    (simple_symbol) @name)
  (#match? @_attr_method "^attr_(accessor|reader|writer)$")) @definition
