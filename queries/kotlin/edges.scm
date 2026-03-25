;; Kotlin edge queries for Inari code intelligence.
;;
;; Pattern order must match the switch cases in KotlinPlugin.ExtractEdge:
;;   0 = import
;;   1 = method call with receiver (navigation_expression)
;;   2 = direct call / constructor call (simple_identifier)
;;   3 = delegation specifier (extends/implements)

;; 0: Import
(import_header
  (identifier) @imported_name) @import_stmt

;; 1: Method call with receiver (e.g. logger.info("hello"))
(call_expression
  (navigation_expression
    (simple_identifier) @receiver
    (navigation_suffix
      (simple_identifier) @callee))) @member_call

;; 2: Direct call or constructor call (e.g. process() or Logger("test"))
(call_expression
  (simple_identifier) @callee) @direct_call

;; 3: Delegation specifier (extends/implements — Kotlin uses : for both)
(delegation_specifier
  (user_type
    (type_identifier) @parent_type)) @delegation_clause
