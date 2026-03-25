;; Go symbol definitions for Inari code intelligence.
;;
;; Each pattern captures a @name (the symbol identifier) and wraps
;; the entire declaration in @definition for line-range extraction.

;; Functions
(function_declaration
  name: (identifier) @name) @definition

;; Methods (with receiver)
(method_declaration
  name: (field_identifier) @name) @definition

;; Type declarations (struct, interface, alias -- single pattern to avoid duplicates)
(type_declaration
  (type_spec
    name: (type_identifier) @name)) @definition

;; Constants
(const_declaration
  (const_spec
    name: (identifier) @name)) @definition
