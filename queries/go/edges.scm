;; Go edge queries for Inari code intelligence.
;;
;; Pattern order must match the switch cases in GoPlugin.ExtractEdge:
;;   0 = import
;;   1 = function call
;;   2 = method call (selector expression)
;;   3 = composite literal (struct instantiation)
;;   4 = type reference in field declaration

;; 0: Import
(import_spec
  path: (interpreted_string_literal) @imported_name) @import

;; 1: Function call
(call_expression
  function: (identifier) @callee) @call

;; 2: Method / qualified call
(call_expression
  function: (selector_expression
    operand: (identifier) @object
    field: (field_identifier) @method)) @member_call

;; 3: Composite literal (struct instantiation)
(composite_literal
  type: (type_identifier) @class_name) @instantiation

;; 4: Type reference in struct field
(field_declaration
  type: (type_identifier) @type_ref) @type_reference
