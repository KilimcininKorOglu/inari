;; 0: REFERENCES table(col) — foreign key
(column_definition (keyword_references) (object_reference name: (identifier) @ref_table)) @fk_ref

;; 1: FROM table — table reference in view/function queries
(from (relation (object_reference name: (identifier) @ref_table))) @from_ref

;; 2: JOIN table — join reference
(join (relation (object_reference name: (identifier) @ref_table))) @join_ref

;; 3: Function call (invocation)
(invocation (object_reference name: (identifier) @func_name)) @func_call
