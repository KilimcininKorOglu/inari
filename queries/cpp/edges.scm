;; 0: #include "file.h" or #include <file.h>
(preproc_include
  path: (_) @source) @include_stmt

;; 1: service->method() or obj.method() — member call
(call_expression
  function: (field_expression
    field: (field_identifier) @callee)) @member_call

;; 2: Class::staticMethod() — scope resolution call
(call_expression
  function: (qualified_identifier
    (identifier) @callee)) @scoped_call

;; 3: new ClassName() — instantiation
(new_expression
  type: (type_identifier) @class_name) @instantiation

;; 4: : public BaseClass — inheritance
(base_class_clause
  (type_identifier) @parent_class) @inheritance

;; 5: direct function call
(call_expression
  function: (identifier) @callee) @direct_call
