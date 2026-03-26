;; Namespace definitions (mapped to module kind)
(namespace_definition
  name: (namespace_name) @name) @definition

;; Class declarations
(class_declaration
  name: (name) @name) @definition

;; Interface declarations
(interface_declaration
  name: (name) @name) @definition

;; Trait declarations (mapped to interface kind)
(trait_declaration
  name: (name) @name) @definition

;; Enum declarations (PHP 8.1)
(enum_declaration
  name: (name) @name) @definition

;; Top-level function definitions
(function_definition
  name: (name) @name) @definition

;; Method declarations (inside class/interface/trait)
(method_declaration
  name: (name) @name) @definition

;; Property declarations (name includes $ prefix — stripped in parser.go)
(property_declaration
  (property_element
    (variable_name) @name)) @definition

;; Class constant declarations (anchor to first child to skip value name)
(const_declaration
  (const_element
    . (name) @name)) @definition
