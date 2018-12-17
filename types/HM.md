# Algorithm 6.16: Type inference for polymorphic functions
<ul>

__INPUT:__ A program consisting of a sequence of function definitions followed by an expression to be evaluated. An expression is made up of function applications and names, where names can have predefined polymorphic types.

__OUTPUT:__ Inferred types for the names in the program.

__METHOD:__ For simplicity, we shall deal with unary functions only. The type of a function _f_(_x<sub>1</sub>_, _x<sub>2</sub>_) with two parameters can be represented by a type expression _s<sub>1</sub>_ × _s<sub>2</sub>_ ⟶ _t_, where _s<sub>1</sub>_ and _s<sub>2</sub>_ are the types of _x<sub>1</sub>_ and _x<sub>2</sub>_, respectively, and _t_ is the type of the result _f_(_x<sub>1</sub>_, _x<sub>2</sub>_). An expression _f_(_a_, _b_) can be checked by matching the type of _a_ with _x<sub>1</sub>_ and the type of _b_ with _x<sub>2</sub>_.
</ul>

Check the function definitions and the expression in the input sequence. Use the inferred type of a function if it is subsequently used in an expression.

* For a function definition __fun id<sub>1</sub>__ (__id<sub>2</sub>__) = _E_, create fresh type variables _α_ and _β_. Associate the type _α_ ⟶ _β_ with the function __id<sub>1</sub>__ , and the type _α_ with the parameter __id<sub>2</sub>__. Then, infer a type for expression _E_. Suppose _α_ denotes type _s_ and _β_ denotes type _t_ after type inference for _E_. The inferred type of function __id<sub>1</sub>__ is _s_ ⟶ _t_. Bind any type variables that remain unconstrained in _s_ ⟶ _t_ by ∀ quantifiers.

* For a function application _E<sub>1</sub>_(_E<sub>2</sub>_), infer types for _E<sub>1</sub>_ and _E<sub>2</sub>_. Since _E<sub>1</sub>_ is used as a function, its type must have the form _s_ ⟶ _s'_. (Technically, the type of _E<sub>1</sub>_ must unify with _β_ ⟶ _γ_, where _β_ and _γ_ are new type variables). Let _t_ be the inferred type of _E<sub>1</sub>_. Unify _s_ and _t_. If unification fails, the expression has a type error. Otherwise, the inferred type of _E<sub>1</sub>_ (_E<sub>2</sub>_) is _s'_ .

* For each occurrence of a polymorphic function, replace the bound vari­ables in its type by distinct fresh variables and remove the ∀ quantifiers. The resulting type expression is the inferred type of this occurrence.

* For a name that is encountered for the first time, introduce a fresh variable for its type.

□

Aho, A. V., Lam, M. S., Sethi, R., & Ullman, J. D. (2007). Type Checking. Compilers: Principles, Techniques, & Tools (2nd ed.) (pp. #393-394). Boston: Pearson/Addison Wesley.