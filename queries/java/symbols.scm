;; Java symbol definitions for Inari code intelligence.
;;
;; Each pattern captures a @name (the symbol identifier) and wraps
;; the entire declaration in @definition for line-range extraction.

;; Classes
(class_declaration
  name: (identifier) @name) @definition

;; Interfaces
(interface_declaration
  name: (identifier) @name) @definition

;; Enums
(enum_declaration
  name: (identifier) @name) @definition

;; Records (Java 16+)
(record_declaration
  name: (identifier) @name) @definition

;; Annotation types
(annotation_type_declaration
  name: (identifier) @name) @definition

;; Methods
(method_declaration
  name: (identifier) @name) @definition

;; Constructors
(constructor_declaration
  name: (identifier) @name) @definition

;; Fields (single declarator to avoid duplicates)
(field_declaration
  declarator: (variable_declarator
    name: (identifier) @name)) @definition
