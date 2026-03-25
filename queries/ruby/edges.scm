;; 0: require / require_relative
(call
  method: (identifier) @_req_method
  arguments: (argument_list
    (string
      (string_content) @source))
  (#match? @_req_method "^require(_relative)?$")) @import_stmt

;; 1: Class inheritance (class Foo < Bar)
(class
  name: (constant) @_class_name
  superclass: (superclass
    (constant) @parent_class)) @inheritance

;; 2: include / extend / prepend mixin
(call
  method: (identifier) @_mixin_method
  arguments: (argument_list
    (constant) @mixin_target)
  (#match? @_mixin_method "^(include|extend|prepend)$")) @mixin_call

;; 3: Method call with receiver (e.g. logger.info("hello"))
(call
  receiver: (_) @receiver
  method: (identifier) @callee) @member_call

;; 4: Direct function/method call without receiver
(call
  method: (identifier) @callee
  !receiver) @direct_call

;; 5: super call
(call
  method: (super) @_super_kw) @super_call
