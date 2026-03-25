;; Java edge queries for Inari code intelligence.
;;
;; Pattern order must match the switch cases in JavaPlugin.ExtractEdge:
;;   0 = import
;;   1 = method call with receiver
;;   2 = method call without receiver
;;   3 = object creation (new)
;;   4 = extends (superclass)
;;   5 = implements (super_interfaces)

;; 0: Import
(import_declaration
  (scoped_identifier) @imported_name) @import_stmt

;; 1: Method call with receiver (e.g. service.process())
(method_invocation
  object: (identifier) @receiver
  name: (identifier) @callee) @member_call

;; 2: Method call without receiver (e.g. process())
(method_invocation
  name: (identifier) @callee
  !object) @direct_call

;; 3: Object creation (new ClassName())
(object_creation_expression
  type: (type_identifier) @class_name) @instantiation

;; 4: Extends (class Foo extends Bar)
(superclass
  (type_identifier) @parent_class) @extends_clause

;; 5: Implements (class Foo implements Bar, Baz)
(super_interfaces
  (type_list
    (type_identifier) @interface_name)) @implements_clause
