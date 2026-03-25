;; 0: #include "file.h" or #include <file.h>
(preproc_include
  path: (_) @source) @include_stmt

;; 1: foo(args) — function call
(call_expression
  function: (identifier) @callee) @direct_call
