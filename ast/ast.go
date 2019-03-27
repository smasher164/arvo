// restrict named interface{} types

package ast

import (
	"io"

	"github.com/smasher164/arvo/scan"
)

type Package struct {
	Files   []*File
	version string // TODO, type.
}

type File struct {
	Comments   *CommentGroup
	Package    PackageDecl
	Decls      []*GenDecl
	Stmts      []Stmt
	Scope      *Scope
	Unresolved []*Ident
	Src        NamedReader
}

type Node interface{}

// maybe every node has positional information?
type Stmt interface{}

type Expr interface{}

type Spec interface{}

type NamedReader interface {
	Name() string
	io.Reader
}

type CommentGroup struct {
	List []*Comment
}

type RelComments struct {
	List map[CommentPos]*Comment
}

type CommentPos int

const (
	Above CommentPos = iota
	Below
	Before
	After
)

type Comment struct {
	Slash scan.Token
	Text  string
}

type PackageDecl struct {
	Comments *RelComments
	Package  scan.Token
	Name     string
}

type GenDecl struct {
	Comments *RelComments
	Keyword  scan.Token
	Lparen   scan.Token
	Specs    []Spec
	Rparen   scan.Token
}

type UseSpec struct {
	Comments *RelComments
	Name     *Ident
	Path     scan.Token
}

type Scope struct {
	Outer *Scope
	Def   map[string]*Object
}

func (s *Scope) Insert(obj *Object) (alt *Object) {
	if alt = s.Def[obj.Name]; alt == nil {
		s.Def[obj.Name] = obj
	}
	return
}

func (s *Scope) Lookup(name string) *Object {
	return s.Def[name]
}

func NewScope(outer *Scope) *Scope {
	return &Scope{outer, make(map[string]*Object)}
}

type Object struct {
	Kind ObjKind
	Name string      // declared name
	Decl interface{} // corresponding Field, XxxSpec, FuncDef, LabeledStmt, AssignStmt, Scope; or nil
	Data interface{} // object-specific data; or nil
	Type interface{} // placeholder for type information; may be nil
}

func (o *Object) Tok() scan.Token {
	name := o.Name
	switch d := o.Decl.(type) {
	case []*Param:
		for _, n := range d {
			if n.Name.Name.Lit == name {
				return n.Name.Name
			}
		}
	case *UseSpec:
		if d.Name != nil && d.Name.Name.Lit == name {
			return d.Name.Name
		}
		return d.Path
	case *ValueSpec:
		for _, n := range d.Names {
			if n.Name.Lit == name {
				return n.Name
			}
		}
	case *FunDef:
		if d.Name.Name.Lit == name {
			return d.Name.Name
		}
	case *LabeledStmt:
		if d.Label.Name.Lit == name {
			return d.Label.Name
		}
	case *AssignStmt:
		for _, x := range d.Lhs {
			if ident, isIdent := x.(*Ident); isIdent && ident.Name.Lit == name {
				return ident.Name
			}
		}
	case *Scope:
	}
	return scan.Token{}
}

func NewObj(kind ObjKind, name string) *Object {
	return &Object{Kind: kind, Name: name}
}

type ValueSpec struct {
	Comments *RelComments
	Names    []*Ident
	Values   []Expr
}

type BinaryExpr struct {
	Comments *RelComments
	X        Expr
	Op       scan.Token
	Y        Expr
}

type UnaryExpr struct {
	Op scan.Token
	X  Expr
}

type ArrayLit struct {
	Comments *RelComments
	A        scan.Token
}

type RecordLit struct {
	Comments *RelComments
	R        scan.Token
}

type BasicLit struct {
	Value scan.Token
}

type BlockStmt struct {
	Comments *RelComments
	Lbrace   scan.Token
	List     []Stmt
	Rbrace   scan.Token
}

type Param struct {
	Comments *RelComments
	Name     *Ident
	Ellipsis scan.Token
}

type FunDef struct {
	Comments *RelComments
	Fun      scan.Token
	Name     *Ident
	Lparen   scan.Token
	Params   []*Param
	Rparen   scan.Token
	Body     *BlockStmt
}

type ParenExpr struct {
	Comments *RelComments
	Lparen   scan.Token
	X        Expr
	Rparen   scan.Token
}

type BadExpr struct {
	Comments *RelComments
	From     scan.Token
	To       scan.Token
}

type SliceExpr struct {
	Comments *RelComments
	X        Expr
	Lbrack   scan.Token
	Low      Expr
	Colon    scan.Token
	High     Expr
	Rbrack   scan.Token
}

type IndexExpr struct {
	Comments  *RelComments
	X         Expr
	LbrackOut scan.Token
	LbrackIn  scan.Token
	Index     Expr
	Backwards bool
	RbrackIn  scan.Token
	RbrackOut scan.Token
}

type SelectorExpr struct {
	Comments *RelComments
	X        Expr
	Sel      *Ident
}

type KeyValueExpr struct {
	Comments *RelComments
	Key      Expr
	Colon    scan.Token
	Value    Expr
}

type CompositeLit struct {
	Comments *RelComments
	Type     Expr
	Lbrace   scan.Token
	Elts     []Expr
	Rbrace   scan.Token
}

type CallExpr struct {
	Comments *RelComments
	Fun      Expr
	Lparen   scan.Token
	Args     []Expr
	Ellipsis scan.Token
	Rparen   scan.Token
}

type AssignStmt struct {
	Comments *RelComments
	Lhs      []Expr
	Tok      scan.Token
	Rhs      []Expr
}

type LabeledStmt struct {
	Comments *RelComments
	Label    *Ident
	Colon    scan.Token
	Stmt     Stmt
}

type BadStmt struct {
	Comments *RelComments
	From     scan.Token
	To       scan.Token
}

type IncDecStmt struct {
	Comments *RelComments
	X        Expr
	Tok      scan.Token
}

type ExprStmt struct {
	Comments *RelComments
	X        Expr
}

type ReturnStmt struct {
	Comments *RelComments
	Return   scan.Token
	Results  []Expr
}

type BranchStmt struct {
	Comments *RelComments
	Tok      scan.Token
	Label    *Ident
}

type IfStmt struct {
	Comments *RelComments
	If       scan.Token
	Init     Stmt
	Cond     Expr
	Body     *BlockStmt
	Else     Stmt
}

type SwitchStmt struct {
	Comments *RelComments
	Switch   scan.Token
	Init     Stmt
	Tag      Expr
	Body     *BlockStmt
}

type CaseClause struct {
	Comments *RelComments
	Case     scan.Token
	List     []Expr
	Colon    scan.Token
	Body     []Stmt
}

type InStmt struct {
	Comments *RelComments
	For      scan.Token
	Index    Expr
	Key      Expr
	Value    Expr
	Tok      scan.Token
	X        Expr
	Body     *BlockStmt
}

type ForStmt struct {
	Comments *RelComments
	For      scan.Token
	Init     Stmt
	Cond     Expr
	Post     Stmt
	Body     *BlockStmt
}

type DeclStmt struct {
	Comments *RelComments
	Decl     *GenDecl
}

type EmptyStmt struct {
	Semicolon scan.Token
	Implicit  bool
}

type ObjKind int

const (
	Bad ObjKind = iota // for error handling
	Pkg                // package
	Con                // constant
	Typ                // type
	Var                // variable
	Fun                // function or method
	Lbl                // label
)

type Ident struct {
	Name scan.Token
	Obj  *Object
}

func Walk(n Node, pre, post WalkFunc) Node {
	if pre != nil && !pre(n) {
		return n
	}
	switch n := n.(type) {
	case nil:
	case *RelComments, *Comment:
	case *BadExpr, *Ident, *BasicLit:
	case *FunDef:
		Walk(n.Comments, pre, post)
		Walk(n.Name, pre, post)
		for _, p := range n.Params {
			Walk(p, pre, post)
		}
		Walk(n.Body, pre, post)
	case *CompositeLit:
		Walk(n.Comments, pre, post)
		Walk(n.Type, pre, post)
		for _, e := range n.Elts {
			Walk(e, pre, post)
		}
	case *ArrayLit:
		Walk(n.Comments, pre, post)
	case *RecordLit:
		Walk(n.Comments, pre, post)
	case *ParenExpr:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
	case *SelectorExpr:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
		Walk(n.Sel, pre, post)
	case *IndexExpr:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
		Walk(n.Index, pre, post)
	case *SliceExpr:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
		Walk(n.Low, pre, post)
		Walk(n.High, pre, post)
	case *CallExpr:
		Walk(n.Comments, pre, post)
		Walk(n.Fun, pre, post)
		for _, e := range n.Args {
			Walk(e, pre, post)
		}
	case *UnaryExpr:
		Walk(n.X, pre, post)
	case *BinaryExpr:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
		Walk(n.Y, pre, post)
	case *KeyValueExpr:
		Walk(n.Comments, pre, post)
		Walk(n.Key, pre, post)
		Walk(n.Value, pre, post)
	case *BadStmt:
	case *DeclStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Decl, pre, post)
	case *EmptyStmt:
	case *LabeledStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Label, pre, post)
		Walk(n.Stmt, pre, post)
	case *ExprStmt:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
	case *IncDecStmt:
		Walk(n.Comments, pre, post)
		Walk(n.X, pre, post)
	case *AssignStmt:
		Walk(n.Comments, pre, post)
		for _, e := range n.Lhs {
			Walk(e, pre, post)
		}
		for _, e := range n.Rhs {
			Walk(e, pre, post)
		}
	case *ReturnStmt:
		Walk(n.Comments, pre, post)
		for _, r := range n.Results {
			Walk(r, pre, post)
		}
	case *BranchStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Label, pre, post)
	case *BlockStmt:
		Walk(n.Comments, pre, post)
		for _, s := range n.List {
			Walk(s, pre, post)
		}
	case *IfStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Init, pre, post)
		Walk(n.Cond, pre, post)
		Walk(n.Body, pre, post)
		Walk(n.Else, pre, post)
	case *CaseClause:
		Walk(n.Comments, pre, post)
		for _, e := range n.List {
			Walk(e, pre, post)
		}
		for _, s := range n.Body {
			Walk(s, pre, post)
		}
	case *SwitchStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Init, pre, post)
		Walk(n.Tag, pre, post)
		Walk(n.Body, pre, post)
	case *ForStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Init, pre, post)
		Walk(n.Cond, pre, post)
		Walk(n.Post, pre, post)
		Walk(n.Body, pre, post)
	case *InStmt:
		Walk(n.Comments, pre, post)
		Walk(n.Index, pre, post)
		Walk(n.Key, pre, post)
		Walk(n.Value, pre, post)
		Walk(n.X, pre, post)
		Walk(n.Body, pre, post)
	case *UseSpec:
		Walk(n.Comments, pre, post)
		Walk(n.Name, pre, post)
	case *ValueSpec:
		Walk(n.Comments, pre, post)
		for _, id := range n.Names {
			Walk(id, pre, post)
		}
		for _, e := range n.Values {
			Walk(e, pre, post)
		}
	case *PackageDecl:
		Walk(n.Comments, pre, post)
	case *GenDecl:
		Walk(n.Comments, pre, post)
		for _, sp := range n.Specs {
			Walk(sp, pre, post)
		}
	case *File:
		Walk(n.Package, pre, post)
		for _, d := range n.Decls {
			Walk(d, pre, post)
		}
		for _, s := range n.Stmts {
			Walk(s, pre, post)
		}
	case *Package:
		for _, f := range n.Files {
			Walk(f, pre, post)
		}
	}
	if post != nil && !post(n) {
		return n
	}
	return n
}

type WalkFunc func(Node) bool
