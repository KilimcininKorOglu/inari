;; Message declarations
(message (message_name (identifier) @name) (message_body)) @definition

;; Enum declarations
(enum (enum_name (identifier) @name) (enum_body)) @definition

;; Service declarations
(service (service_name (identifier) @name)) @definition

;; RPC methods
(rpc (rpc_name (identifier) @name)) @definition

;; Enum fields (constants)
(enum_field (identifier) @name) @definition

;; Message fields (properties)
(field (identifier) @name) @definition

;; Map fields (properties)
(map_field (identifier) @name) @definition
