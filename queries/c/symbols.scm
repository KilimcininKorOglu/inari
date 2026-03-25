;; Function definitions
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @name)) @definition

;; Pointer-returning function definitions (e.g. Foo* create())
(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @name))) @definition

;; Named struct definitions (struct Foo { })
(struct_specifier
  name: (type_identifier) @name
  body: (field_declaration_list)) @definition

;; Named enum definitions (enum Bar { })
(enum_specifier
  name: (type_identifier) @name
  body: (enumerator_list)) @definition

;; Typedef definitions (typedef ... Foo;)
(type_definition
  declarator: (type_identifier) @name) @definition
