;; CREATE TABLE
(create_table (object_reference name: (identifier) @name) (column_definitions)) @definition

;; CREATE VIEW
(create_view (object_reference name: (identifier) @name)) @definition

;; CREATE FUNCTION
(create_function (object_reference name: (identifier) @name)) @definition

;; Column definitions (properties)
(column_definition name: (identifier) @name) @definition
