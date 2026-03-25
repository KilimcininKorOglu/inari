;; 0: import "file.proto"
(import path: (string) @source) @import_stmt

;; 1: field type reference (message_or_enum_type)
(field (type (message_or_enum_type) @type_ref)) @field_ref

;; 2: rpc request/response type reference
(rpc (message_or_enum_type) @type_ref) @rpc_ref

;; 3: oneof field type reference
(oneof_field (type (message_or_enum_type) @type_ref)) @oneof_ref

;; 4: map field value type reference
(map_field (type (message_or_enum_type) @type_ref)) @map_ref
