;; Class declarations
(class_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition

;; Struct declarations
(struct_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition

;; Enum declarations
(enum_specifier
  name: (type_identifier) @name
  body: (enumerator_list)) @definition

;; Namespace definitions
(namespace_definition
  name: (namespace_identifier) @name) @definition

;; Function definitions (top-level)
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @name)) @definition

;; Qualified function definitions (Class::method style)
(function_definition
  declarator: (function_declarator
    declarator: (qualified_identifier
      name: (identifier) @name))) @definition

;; Pointer-returning function definitions
(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @name))) @definition

;; Pointer-returning qualified function definitions
(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (qualified_identifier
        name: (identifier) @name)))) @definition

;; Typedef definitions
(type_definition
  declarator: (type_identifier) @name) @definition
