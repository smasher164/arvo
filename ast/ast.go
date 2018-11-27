// restrict named interface{} types

package ast

import (
	"io"

	"github.com/smasher164/arvo/scan"
)

type Package struct {
	files   []*File
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
	Decl interface{} // corresponding Field, XxxSpec, FuncDecl, LabeledStmt, AssignStmt, Scope; or nil
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
	Lbrace scan.Token
	List   []Stmt
	Rbrace scan.Token
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
	Comments  *RelComments
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
