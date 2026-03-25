;; 0: source ./file.sh or . ./file.sh — import
(command
  name: (command_name
    (word) @_cmd)
  (word) @source) @import_cmd

;; 1: Direct command call (function call)
(command
  name: (command_name
    (word) @callee)) @direct_call
