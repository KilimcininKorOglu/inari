;; Global/dot/colon function — captures last identifier in function_name
;; Handles: function foo(), function Foo.bar(), function Foo:baz()
(function_statement
  (function_name
    (identifier) @name .)) @definition

;; Local function — local function helper()
;; Name is a direct identifier child (not inside function_name)
(function_statement
  (local)
  (identifier) @name) @definition
