;; 0: require("module") — import
;; Captures the full string node; quotes stripped in ExtractEdge.
(function_call
  (identifier) @callee
  (function_arguments
    (string) @source)) @import_call

;; 1: obj:method() — colon method call
(function_call
  (identifier) @receiver
  (self_call_colon)
  (identifier) @callee) @colon_call

;; 2: module.func() — dot call (two identifiers without colon)
(function_call
  (identifier) @receiver
  (identifier) @callee) @dot_call

;; 3: func() — direct function call
(function_call
  (identifier) @callee) @direct_call
