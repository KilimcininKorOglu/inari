;; Class, struct, and enum declarations (all use class_declaration node type)
;; Distinguished by first child keyword — stored in metadata, not in kind
(class_declaration
  (type_identifier) @name) @definition

;; Protocol declarations
(protocol_declaration
  (type_identifier) @name) @definition

;; Function declarations
(function_declaration
  (simple_identifier) @name) @definition

;; Protocol function declarations
(protocol_function_declaration
  (simple_identifier) @name) @definition

;; Property declarations (let/var)
(property_declaration
  (pattern
    (simple_identifier) @name)) @definition
