package types

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/smasher164/arvo/ast"
	"github.com/smasher164/arvo/scan"
)

type Config struct {
	File   *ast.File
	Types  map[ast.Expr]Type
	retstk []Tuple
}

func (c *checker) pushret(r Tuple) {
	c.conf.retstk = append(c.conf.retstk, r)
}

func (c *checker) popret() (r Tuple) {
	if len(c.conf.retstk) > 0 {
		r, c.conf.retstk = c.conf.retstk[len(c.conf.retstk)-1], c.conf.retstk[:len(c.conf.retstk)-1]
	}
	return
}

type Type interface{}

type checker struct {
	err  errorlist
	conf *Config
}

type errorlist []error

func (e errorlist) Error() string {
	var sb strings.Builder
	for i := range e {
		sb.WriteString(e[i].Error())
		if i != len(e)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func (c *checker) errorf(format string, args ...interface{}) {
	c.err = append(c.err, fmt.Errorf(format, args...))
}

type Basic int

const (
	Bool Basic = iota
	Num
	String
)

type Record struct {
	N    int
	Elts []Element
}

type Array struct {
	Key, Value Type
}

type Element struct {
	Key, Value Type
}

type Tuple []Type

type Signature struct {
	ParamLen  int
	Params    []Type
	ResultLen int
	Results   []Type
	Variadic  bool
}

type Same *Type

type Invocation struct {
	ArgLen int
	Args   []Type
	Spread bool
	Sig    *Signature
}

type Label struct {
	Obj *ast.Object
}

type or struct {
	A, B Type
}

func Match(a, b Type) bool {
	return match(a, b)
}

func match(a, b Type) bool {
	switch ta := a.(type) {
	case or:
		return match(ta.A, b) || match(ta.B, b)
	case Same:
		if ta != nil {
			return match(*ta, b)
		}
		return reflect.DeepEqual(a, b)
	case nil:
		return true
	}
	switch tb := b.(type) {
	case or:
		return match(a, tb.A) || match(a, tb.B)
	case Same:
		if tb != nil {
			return match(a, *tb)
		}
		return reflect.DeepEqual(a, b)
	case nil:
		return true
	}
	return reflect.DeepEqual(a, b)
}

func (c *checker) set(n ast.Node, t Type) {
	switch n := n.(type) {
	case *ast.Ident:
		if n != nil {
			if n.Obj != nil {
				if n.Obj.Kind == ast.Fun {
					if f, _ := n.Obj.Decl.(*ast.FunDef); f != nil {
						// maybe an issue with anonymous functions?
						c.conf.Types[f.Name] = t
					}
				} else if n.Obj.Kind == ast.Var {
					if pr, _ := n.Obj.Decl.(*ast.Param); pr != nil {
						c.conf.Types[pr.Name] = t
					} else if as, _ := n.Obj.Decl.(*ast.AssignStmt); as != nil {
						for _, e := range as.Lhs {
							if id, _ := e.(*ast.Ident); id != nil {
								if id.Name.Lit == n.Name.Lit {
									c.conf.Types[id] = t
									break
								}
							}
						}
					}
				}
			} else {
				c.conf.Types[n] = t
			}
		}
		c.conf.Types[n] = t
	default:
		c.conf.Types[n] = t
	}
}

func (c *Config) Get(n ast.Node) Type {
	switch n := n.(type) {
	case *ast.Ident:
		if n != nil {
			if n.Obj != nil {
				if n.Obj.Kind == ast.Fun {
					if f, _ := n.Obj.Decl.(*ast.FunDef); f != nil {
						return c.Types[f.Name]
					}
				} else if n.Obj.Kind == ast.Var {
					if pr, _ := n.Obj.Decl.(*ast.Param); pr != nil {
						return c.Types[pr.Name]
					} else if as, _ := n.Obj.Decl.(*ast.AssignStmt); as != nil {
						for _, e := range as.Lhs {
							if id, _ := e.(*ast.Ident); id != nil {
								if id.Name.Lit == n.Name.Lit {
									return c.Types[id]
								}
							}
						}
					}
				}
			} else {
				return c.Types[n]
			}
		}
		return c.Types[n]
	default:
		return c.Types[n]
	}
}

func (c *checker) get(n ast.Node) Type {
	switch n := n.(type) {
	case *ast.Ident:
		if n != nil {
			if n.Obj != nil {
				if n.Obj.Kind == ast.Fun {
					if f, _ := n.Obj.Decl.(*ast.FunDef); f != nil {
						return c.conf.Types[f.Name]
					}
				} else if n.Obj.Kind == ast.Var {
					if pr, _ := n.Obj.Decl.(*ast.Param); pr != nil {
						return c.conf.Types[pr.Name]
					} else if as, _ := n.Obj.Decl.(*ast.AssignStmt); as != nil {
						for _, e := range as.Lhs {
							if id, _ := e.(*ast.Ident); id != nil {
								if id.Name.Lit == n.Name.Lit {
									return c.conf.Types[id]
								}
							}
						}
					}
				}
			} else {
				return c.conf.Types[n]
			}
		}
		return c.conf.Types[n]
	default:
		return c.conf.Types[n]
	}
}

func (c *checker) eval(t Type) Type {
	switch t := t.(type) {
	case Invocation:
		if t.Sig != nil && len(t.Sig.Results) != 0 {
			if len(t.Sig.Results) == 1 {
				return t.Sig.Results[0]
			} else {
				return Tuple(t.Sig.Results)
			}
		}
		return nil
	case or:
		return or{c.eval(t.A), c.eval(t.B)}
	default:
		return t
	}
}

func assert(cond bool, msg string) {
	if !cond {
		panic(msg)
	}
}

func getCalledDef(e *ast.CallExpr) *ast.FunDef {
	switch t := e.Fun.(type) {
	case *ast.Ident:
		if t2, _ := t.Obj.Decl.(*ast.FunDef); t2 != nil {
			return t2
		}
	case *ast.FunDef:
		return t
	}
	return nil
}

// on the way down
func (c *checker) pre(n ast.Node) bool {
	switch t := n.(type) {
	// case *RelComments, *Comment:
	// case *ast.Ident:
	// case *BadExpr:
	case *ast.BasicLit:
		switch t.Value.Type {
		case scan.Ident:
			if t.Value.Lit == "true" || t.Value.Lit == "false" {
				c.set(t, Bool)
			}
		case scan.Int, scan.Float:
			c.set(t, Num)
		case scan.String:
			c.set(t, String)
		}
	case *ast.FunDef:
		var variadic bool
		for _, el := range t.Params {
			if el != nil && el.Ellipsis.Type == scan.Ellipsis {
				variadic = true
			}
		}
		c.set(t, Signature{ParamLen: len(t.Params), Params: make([]Type, len(t.Params)), Variadic: variadic})
		if t.Name != nil {
			c.set(t.Name, Signature{ParamLen: len(t.Params), Params: make([]Type, len(t.Params)), Variadic: variadic})
		}
		c.pushret(nil)
	case *ast.CompositeLit:
		switch t.Type.(type) {
		case *ast.ArrayLit:
			c.set(t, Array{})
		case *ast.RecordLit:
			n := len(t.Elts)
			c.set(t, Record{N: n, Elts: make([]Element, n)})
		}
	// case *ast.ArrayLit:
	// case *ast.RecordLit:
	// case *ast.ParenExpr:
	// case *ast.SelectorExpr:
	// case *ast.IndexExpr:
	// case *ast.SliceExpr:
	case *ast.CallExpr:
		c.set(t, Invocation{ArgLen: len(t.Args), Spread: t.Ellipsis.Type == scan.Ellipsis})
	// case *ast.UnaryExpr:
	// case *ast.BinaryExpr:
	case *ast.KeyValueExpr:
		c.set(t, Element{})
	// case *BadStmt:
	// case *ast.DeclStmt:
	// case *EmptyStmt:
	// case *ast.LabeledStmt:
	// case *ast.ExprStmt:
	case *ast.IncDecStmt:
		c.set(t.X, Num)
	case *ast.AssignStmt:
		if len(t.Lhs) != len(t.Rhs) {
			c.errorf("left-hand side and right-hand side do not match: %d %s %d", len(t.Lhs), t.Tok, len(t.Rhs))
		}
	// case *ast.ReturnStmt:
	case *ast.BranchStmt:
		if t.Label != nil {
			c.set(t.Label, Label{Obj: t.Label.Obj})
		}
	// case *ast.BlockStmt:
	// case *ast.IfStmt:
	// case *ast.CaseClause:
	// case *ast.SwitchStmt:
	// case *ast.ForStmt:
	case *ast.InStmt:
		c.set(t.X, or{Array{}, Record{}})
	// case *UseSpec:
	case *ast.ValueSpec:
		if len(t.Names) != len(t.Values) {
			c.errorf("left-hand side and right-hand side do not match: %d = %d", len(t.Names), len(t.Values))
		}
	// case *PackageDecl:
	// case *ast.GenDecl:
	case *ast.File:
		// case *Package:
	}
	return true
}

// on the way up
func (c *checker) post(n ast.Node) bool {
	switch t := n.(type) {
	// case *RelComments, *Comment:
	// case *ast.Ident:
	// case *BadExpr:
	// case *ast.BasicLit:
	case *ast.FunDef:
		sig, ok := c.get(t).(Signature)
		assert(ok, "function definition contains signature")
		for i := range sig.Params {
			sig.Params[i] = c.get(t.Params[i].Name)
		}
		if sig.Variadic {
			sig.Params[len(sig.Params)-1] = Array{Key: Num, Value: sig.Params[len(sig.Params)-1]}
		}
		sig.Results = c.popret()
		sig.ResultLen = len(sig.Results)
		c.set(t, sig)
		c.set(t.Name, sig)
	case *ast.CompositeLit:
		switch t.Type.(type) {
		case *ast.ArrayLit:
			if len(t.Elts) != 0 {
				var errd bool
				t0 := c.get(t.Elts[0])
				for i := 1; i < len(t.Elts); i++ {
					if t0 != c.get(t.Elts[1]) {
						c.errorf("array holds values of varying type")
						errd = true
					}
				}
				if !errd {
					if el, ok := t0.(Element); ok {
						c.set(t, Array{Key: c.eval(el.Key), Value: c.eval(el.Value)})
					} else {
						c.set(t, Array{Key: Num, Value: c.eval(t0)})
					}
				}
			}
		case *ast.RecordLit:
			e := c.get(t).(Record).Elts
			for i := range t.Elts {
				t0 := c.get(t.Elts[i])
				if el, ok := t0.(Element); ok {
					e[i] = Element{Key: el.Key, Value: el.Value}
				} else {
					e[i] = Element{Key: Num, Value: t0}
				}
			}
		}
	// case *ast.ArrayLit:
	// case *ast.RecordLit:
	case *ast.ParenExpr:
		c.set(t, c.get(t.X))
	case *ast.SelectorExpr:
		// c.conf.Types[t] = Selection{A: c.conf.Types[t.X], B: c.conf.Types[t.Sel]}
		c.set(t, c.get(t.Sel))
	case *ast.IndexExpr:
		if t.Backwards {
			switch t0 := c.get(t.X).(type) {
			case Array:
				if t0.Value != nil && c.get(t.Index) != nil && t0.Value != c.get(t.Index) {
					c.errorf("array value and index types do not match")
				}
				c.set(t, Array{Key: Num, Value: c.eval(c.get(t.Index))})
			case Record:
				c.errorf("record cannot be reverse-indexed")
			}
		} else {
			switch t0 := c.get(t.X).(type) {
			case Array:
				if t0.Key != nil && c.get(t.Index) != nil && t0.Key != c.get(t.Index) {
					c.errorf("array key and index types do not match")
				}
			case Record:
				id := c.get(t.Index)
				var found bool
				for i := range t0.Elts {
					k := t0.Elts[i].Key
					if k == id {
						found = true
						break
					}
				}
				if !found {
					c.errorf("record key and index types do not match")
				}
			}
		}
	case *ast.SliceExpr:
		switch c.get(t.X).(type) {
		case Array:
			if c.get(t.Low) != Num && c.get(t.High) != Num {
				c.errorf("slice bounds must be numbers")
			}
			c.set(t, c.eval(c.get(t.X)))
		case Record:
			c.errorf("record cannot be sliced")
		}
	case *ast.CallExpr:
		inv, ok := c.get(t).(Invocation)
		assert(ok, "cannot retrieve function invocation")
		sig, ok := c.get(t.Fun).(Signature)
		assert(ok, "cannot retrieve function declaration")
		if !sig.Variadic && inv.ArgLen != sig.ParamLen {
			c.errorf("number of arguments does not match number of parameters")
			break
		}
		if sig.Variadic && inv.ArgLen < (sig.ParamLen-1) {
			c.errorf("number of arguments does not match number of parameters")
			break
		}
		n := sig.ParamLen
		if sig.Variadic {
			n--
		}
		for i := 0; i < n; i++ {
			at := c.get(t.Args[i])
			inv.Args = append(inv.Args, at)
			typ := c.eval(sig.Params[i])
			if !match(at, c.eval(sig.Params[i])) {
				c.errorf("argument types don't match parameter types")
				break
			}
			if typ == nil {
				if ce := getCalledDef(t); ce != nil {
					ot := or{typ, at}
					c.set(ce.Params[i], ot)
					// reflow function's type
					sig.Params[i] = ot
				}
			}
		}
		if sig.Variadic {
			ar, ok := sig.Params[len(sig.Params)-1].(Array)
			if !ok && inv.ArgLen >= sig.ParamLen {
				panic("variadic parameter must have type array")
			}
			t1 := ar.Value
			for i := n; i < inv.ArgLen; i++ {
				at := c.get(t.Args[i])
				inv.Args = append(inv.Args, at)
				if !match(at, t1) {
					c.errorf("argument types don't match parameter types")
					break
				}
			}
		}
		inv.Sig = &sig
		c.set(t, inv)
	case *ast.UnaryExpr:
		// if t.X has a type and is not a number or bool, error
		typ := c.get(t.X)
		if typ != nil {
			if t0, ok := typ.(Basic); !ok || t0 == String {
				c.errorf("unary operation can only be performed on number or bool")
				break
			}
		} else {
			typ = or{Bool, Num}
		}
		typ = c.eval(typ)
		c.set(t, typ)
		c.set(t.X, typ)
	case *ast.BinaryExpr:
		tx, ty := c.get(t.X), c.get(t.Y)
		var typ Type
		if t.Op.Type == scan.Add || t.Op.Type == scan.Eql || t.Op.Type == scan.Lss ||
			t.Op.Type == scan.Gtr || t.Op.Type == scan.Neq ||
			t.Op.Type == scan.Leq || t.Op.Type == scan.Geq {
			if tx != nil || ty != nil {
				typ = or{tx, ty}
			} else {
				typ = or{or{Bool, Num}, String}
			}
		} else {
			if tx != nil && ty != nil {
				tx0, ok1 := tx.(Basic)
				ty0, ok2 := ty.(Basic)
				if (!ok1 || tx0 == String) || (!ok2 || ty0 == String) {
					c.errorf("binary operation can only be performed between numbers or bools")
					break
				} else {
					typ = or{tx, ty}
				}
			} else {
				typ = or{Bool, Num}
			}
		}
		typ = c.eval(typ)
		c.set(t, typ)
		c.set(t.X, typ)
		c.set(t.Y, Same(&typ))
	case *ast.KeyValueExpr:
		c.set(t, Element{Key: c.eval(c.get(t.Key)), Value: c.eval(c.get(t.Value))})
	// case *BadStmt:
	case *ast.DeclStmt:
	// case *EmptyStmt:
	// case *ast.LabeledStmt:
	// case *ast.ExprStmt:
	case *ast.IncDecStmt:
		// if t.X has a type and is not a number or bool, error
		if typ := c.get(t.X); typ != nil {
			if t0, ok := typ.(Basic); !ok || t0 == String {
				c.errorf("can only increment and decrement a number or bool")
			}
		}
	case *ast.AssignStmt:
		if t.Tok.Type != scan.Assign && (len(t.Lhs) > 1 || len(t.Rhs) > 1) {
			c.errorf("assignment operator can only operate on one element on lhs and rhs")
			break
		}
		if t.Tok.Type != scan.Assign {
			if c.get(t.Lhs[0]) != nil && !match(c.get(t.Lhs[0]), c.get(t.Rhs[0])) {
				c.errorf("lhs does not match rhs type")
				break
			}
			c.set(t.Lhs[0], c.eval(c.get(t.Rhs[0])))
		} else {
			for i := range t.Lhs {
				if t0 := c.get(t.Lhs[i]); t0 != nil {
					if !match(t0, c.eval(c.get(t.Rhs[i]))) {
						fmt.Println("HEREAFTER")
						c.errorf("lhs does not match rhs type")
					} else {
						c.set(t.Lhs[i], c.eval(c.get(t.Rhs[i])))
					}
				} else {
					c.set(t.Lhs[i], c.eval(c.get(t.Rhs[i])))
				}
			}
		}
	case *ast.ReturnStmt:
		tuple := c.conf.retstk[len(c.conf.retstk)-1]
		if tuple == nil {
			for i := range t.Results {
				tuple = append(tuple, c.eval(c.get(t.Results[i])))
			}
			c.popret()
			c.pushret(tuple)
			break
		}
		// if result types list exist and types of results don't match their respective signature result types, error
		if len(tuple) != len(t.Results) {
			c.errorf("number of return values do not match")
			break
		}
		for i, t1 := range tuple {
			if !match(c.get(t.Results[i]), t1) {
				c.errorf("return statement does not match signature")
			}
		}
	case *ast.BranchStmt:
	// case *ast.BlockStmt:
	// case *ast.IfStmt:
	// case *ast.CaseClause:
	case *ast.SwitchStmt:
		// figure out type of tag
		var typ Type = Bool
		if t.Tag != nil {
			typ = c.get(t.Tag)
		}
		// make other types point to that type
		for i := range t.Body.List {
			s, _ := t.Body.List[i].(*ast.CaseClause)
			if s != nil {
				for j := range s.List {
					e := s.List[j]
					if t2 := c.get(e); t2 != nil && t2 != typ {
						c.errorf("case expressions must match switch tag type")
					} else {
						c.set(e, Same(&typ))
					}
				}
			}
		}
	// case *ast.ForStmt:
	// case *ast.InStmt:
	// case *UseSpec:
	case *ast.ValueSpec:
		if len(t.Values) > 0 && len(t.Names) != len(t.Values) {
			c.errorf("Lhs and Rhs do not match")
			break
		}
		for i := range t.Names {
			c.set(t.Names[i], c.eval(c.get(t.Values[i])))
		}
	// case *PackageDecl:
	// case *ast.GenDecl:
	case *ast.File:
		// case *Package:
	}
	return true
}

// Type checker performs inference and validation over the syntax tree using
// only a single traversal. The resulting types of expressions are stored in
// the Config's Types map.
func (c *checker) check() error {
	ast.Walk(c.conf.File, c.pre, c.post)
	if len(c.err) == 0 {
		return nil
	}
	return c.err
}

func Infer(conf *Config) error {
	// ignore package and use declarations. so just worry about statements
	if conf == nil {
		panic("unexpected nil config")
	}
	if conf.Types == nil {
		conf.Types = make(map[ast.Expr]Type)
	}
	return (&checker{conf: conf}).check()
}
