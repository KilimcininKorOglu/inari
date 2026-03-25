;; Kotlin symbol definitions for Inari code intelligence.
;;
;; Each pattern captures a @name (the symbol identifier) and wraps
;; the entire declaration in @definition for line-range extraction.
;;
;; NOTE: Kotlin tree-sitter grammar uses class_declaration for both
;; classes and interfaces (distinguished by keyword child node).

;; Classes and interfaces (both use class_declaration)
(class_declaration
  (type_identifier) @name) @definition

;; Object declarations (singletons)
(object_declaration
  (type_identifier) @name) @definition

;; Functions
(function_declaration
  (simple_identifier) @name) @definition

;; Properties (val/var)
(property_declaration
  (variable_declaration
    (simple_identifier) @name)) @definition
