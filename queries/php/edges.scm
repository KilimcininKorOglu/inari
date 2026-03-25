;; 0: use Foo\Bar; (namespace use declaration — import)
(namespace_use_declaration
  (namespace_use_clause
    (qualified_name) @imported_name)) @import_stmt

;; 1: $obj->method() (member call)
(member_call_expression
  object: (_) @receiver
  name: (name) @callee) @member_call

;; 2: ClassName::staticMethod() (scoped/static call)
(scoped_call_expression
  scope: (_) @receiver
  name: (name) @callee) @static_call

;; 3: new ClassName() (object creation)
(object_creation_expression
  (qualified_name
    (name) @class_name)) @instantiation

;; 4: extends ParentClass
(base_clause
  (name) @parent_class) @extends_clause

;; 5: implements InterfaceName
(class_interface_clause
  (name) @interface_name) @implements_clause

;; 6: use TraitName; (inside class body — trait use)
(use_declaration
  (name) @trait_name) @trait_use

;; 7: Direct function call (e.g. array_map(...), myFunction())
(function_call_expression
  function: (name) @callee) @direct_call
