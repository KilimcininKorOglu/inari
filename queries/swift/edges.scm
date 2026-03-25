;; 0: import Foundation
(import_declaration
  (identifier
    (simple_identifier) @imported_name)) @import_stmt

;; 1: service.processPayment() or PaymentService.createDefault() — method/static call
(call_expression
  (navigation_expression
    (simple_identifier) @receiver
    (navigation_suffix
      (simple_identifier) @callee))) @member_call

;; 2: PaymentService(...) or directFunc() — constructor or direct call
(call_expression
  (simple_identifier) @callee) @direct_call

;; 3: : BaseService, PaymentInterface — inheritance
(inheritance_specifier
  (user_type
    (type_identifier) @parent_type)) @inheritance_clause
