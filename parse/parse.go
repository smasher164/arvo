// resolve identifiers
// add tests
// parse comments
// parse package

package parse

import (
	"bufio"
	"errors"
	"fmt"

	"github.com/smasher164/arvo/ast"
	"github.com/smasher164/arvo/scan"
)

type parser struct {
	sc         *scan.Scanner
	tok        scan.Token
	unresolved []*ast.Ident
	topScope   *ast.Scope
	pkgScope   *ast.Scope
	errors     []Error
	inRhs      bool
}

func (p *parser) next() {
	p.tok = p.sc.Scan()
	for p.tok.Type == scan.Comment {
		// TODO(akhil): Parse and store comment state
		p.tok = p.sc.Scan()
	}
}

type Error struct {
	Offset int
	Line   int
	Column int
	Msg    string
}

func (e Error) Error() string {
	return fmt.Sprintf("%d:%d:%d: %s", e.Offset, e.Line, e.Column, e.Msg)
}

func (p *parser) error(tok scan.Token, msg string) {
	p.errors = append(p.errors, Error{tok.Offset, tok.Line, tok.Column, msg})
}

func (p *parser) expectSemi() {
	if p.tok.Type != scan.Rparen && p.tok.Type != scan.Rbrace {
		switch p.tok.Type {
		case scan.Comma:
			p.error(p.tok, "expected ;")
			p.next()
		case scan.Semicolon:
			p.next()
		default:
			p.error(p.tok, "expected ;")
		}
	}
}

func (p *parser) expect(typ scan.Type) scan.Token {
	tok := p.tok
	if tok.Type != typ {
		p.error(tok, "'"+typ.String()+"'")
	}
	p.next()
	return tok
}

func (p *parser) ident() *ast.Ident {
	id := &ast.Ident{Name: p.tok}
	if p.tok.Type == scan.Ident {
		p.next()
		return id
	}
	id.Name.Lit = "_"
	p.expect(scan.Ident)
	return id
}

// IdentifierList = identifier { "," identifier } .
func (p *parser) identList() (list []*ast.Ident) {
	list = append(list, p.ident())
	for p.tok.Type == scan.Comma {
		p.next()
		list = append(list, p.ident())
	}
	return
}

func (p *parser) openScope() {
	p.topScope = ast.NewScope(p.topScope)
}

func (p *parser) closeScope() {
	p.topScope = p.topScope.Outer
}

// UseSpec       = [ "." | PackageName ] UsePath .
func (p *parser) useSpec(_ int) ast.Spec {
	var us ast.UseSpec
	tok := p.tok
	switch p.tok.Type {
	case scan.Period:
		tok.Lit = "."
		us.Name = &ast.Ident{Name: tok}
		p.next()
	case scan.Ident:
		us.Name = p.ident()
	}
	us.Path = p.tok
	p.expect(scan.String)
	return &us
}

func (p *parser) genDecl(keyword scan.Type, f func(int) ast.Spec) *ast.GenDecl {
	var gd ast.GenDecl
	gd.Keyword = p.expect(keyword)
	if p.tok.Type == scan.Lparen {
		gd.Lparen = p.tok
		p.next()
		for i := 0; p.tok.Type != scan.Rparen && p.tok.Type != scan.EOF; i++ {
			gd.Specs = append(gd.Specs, f(i))
		}
		gd.Rparen = p.expect(scan.Rparen)
		p.expectSemi()
	} else {
		gd.Specs = append(gd.Specs, f(0))
	}
	return &gd
}

const (
	basic = iota
	labelOk
	inOk
)

// VarSpec = IdentifierList [ "=" ExpressionList ] .
func (p *parser) valueSpec(i int) ast.Spec {
	idents := p.identList()
	var values []ast.Expr
	if p.tok.Type == scan.Assign {
		p.next()
		values = p.rhsList()
	}
	p.expectSemi()
	spec := &ast.ValueSpec{
		Names:  idents,
		Values: values,
	}
	p.declare(spec, i, p.topScope, ast.Var, idents...)
	return spec
}

func (p *parser) rhs() ast.Expr {
	old := p.inRhs
	p.inRhs = true
	x := p.expr(false)
	p.inRhs = old
	return x
}

func (p *parser) lhsList() []ast.Expr {
	old := p.inRhs
	p.inRhs = false
	list := p.exprList(true)
	// should we resolve idents here?
	p.inRhs = old
	return list
}

func (p *parser) rhsList() []ast.Expr {
	old := p.inRhs
	p.inRhs = true
	list := p.exprList(false)
	p.inRhs = old
	return list
}

// ExpressionList = Expression { "," Expression } .
func (p *parser) exprList(lhs bool) (list []ast.Expr) {
	list = append(list, p.expr(lhs))
	for p.tok.Type == scan.Comma {
		p.next()
		list = append(list, p.expr(lhs))
	}
	return list
}

// Expression = UnaryExpr | Expression binary_op Expression .
func (p *parser) expr(lhs bool) ast.Expr {
	return p.binaryExpr(lhs, scan.LowestPrec+1)
}

func (p *parser) typPrec() (scan.Type, int) {
	typ := p.tok.Type
	if p.inRhs && typ == scan.Assign {
		typ = scan.Eql
	}
	return typ, typ.Precedence()
}

func (p *parser) binaryExpr(lhs bool, prec1 int) ast.Expr {
	x := p.unaryExpr(lhs)
	for {
		op, oprec := p.typPrec()
		if oprec < prec1 {
			return x
		}
		tok := p.expect(op)
		if lhs {
			// p.resolve(x)
			lhs = false
		}
		y := p.binaryExpr(false, oprec+1)
		x = &ast.BinaryExpr{X: x, Op: tok, Y: y}
	}
}

// UnaryExpr = PrimaryExpr | unary_op UnaryExpr .
func (p *parser) unaryExpr(lhs bool) ast.Expr {
	switch p.tok.Type {
	case scan.Add, scan.Sub, scan.Not, scan.Xor, scan.And:
		op := p.tok
		p.next()
		x := p.unaryExpr(false)
		return &ast.UnaryExpr{Op: op, X: x}
	}
	return p.primaryExpr(lhs)
}

// ArrayLit = "a" AssocLit .
func (p *parser) arrayLit() *ast.ArrayLit {
	tok := p.tok
	if tok.Type != scan.Ident && tok.Lit != "a" {
		p.error(tok, "'"+scan.Ident.String()+"'")
	}
	p.next()
	if p.tok.Type != scan.Lbrace {
		p.error(tok, "'"+scan.Lbrace.String()+"'")
	}
	return &ast.ArrayLit{A: tok}
}

// RecordLit = "r" AssocLit .
func (p *parser) recordLit() *ast.RecordLit {
	tok := p.tok
	if tok.Type != scan.Ident && tok.Lit != "r" {
		p.error(tok, "'"+scan.Ident.String()+"'")
	}
	p.next()
	if p.tok.Type != scan.Lbrace {
		p.error(tok, "'"+scan.Lbrace.String()+"'")
	}
	return &ast.RecordLit{R: tok}
}

func (p *parser) body(scope *ast.Scope) *ast.BlockStmt {
	lbrace := p.expect(scan.Lbrace)
	p.topScope = scope
	//p.openLabelScope()
	list := p.stmtList()
	//p.closeLabelScope()
	p.closeScope()
	rbrace := p.expect(scan.Rbrace)
	return &ast.BlockStmt{Lbrace: lbrace, List: list, Rbrace: rbrace}
}

// Parameters = "(" [ ParameterList [ "," ] ] ")" .
// ParameterList = ParameterDecl { "," ParameterDecl } .
// ParameterDecl = [ "..." ] identifier .
func (p *parser) parameterList(scope *ast.Scope) []*ast.Param {
	var list []*ast.Param
	nellipsis := 0
	for p.tok.Type != scan.Rparen && p.tok.Type != scan.EOF {
		var pr ast.Param
		if p.tok.Type == scan.Ellipsis {
			nellipsis++
			pr.Ellipsis = p.tok
			p.next()
		}
		pr.Name = p.ident()
		if p.tok.Type == scan.Comma {
			p.next()
		}
		list = append(list, &pr)
	}
	if nellipsis > 1 || (nellipsis > 0 && list[len(list)-1].Ellipsis.Type == scan.Ellipsis) {
		p.error(p.tok, fmt.Sprintf("can only use ... with final parameter in list"))
	}
	return list
}

// FunctionDecl = "fun" FunctionName Function .
// FunctionLit = "fun" Function .
// FunctionName = identifier .
// Function = Parameters FunctionBody .
// FunctionBody = Block .
func (p *parser) funLit() *ast.FunDef {
	tok := p.expect(scan.Fun)
	scope := ast.NewScope(p.topScope)
	var params []*ast.Param
	var name *ast.Ident
	if p.tok.Type == scan.Ident {
		name = p.ident()
	}
	lparen := p.expect(scan.Lparen)
	if p.tok.Type != scan.Rparen {
		params = p.parameterList(scope)
	}
	rparen := p.expect(scan.Rparen)
	//p.exprLev++
	body := p.body(scope)
	//p.exprLev--
	return &ast.FunDef{Fun: tok, Name: name, Lparen: lparen, Params: params, Rparen: rparen, Body: body}
}

// Operand = Literal | OperandName | "(" Expression ")" .
// Literal = BasicLit | ArrayLit | RecordLit | FunctionLit .
// BasicLit = int_lit | float_lit | string_lit .
// OperandName = identifier .
func (p *parser) operand(lhs bool) (x ast.Expr) {
	switch p.tok.Type {
	case scan.Ident:
		if p.tok.Lit == "a" {
			x = p.arrayLit()
		} else if p.tok.Lit == "r" {
			x = p.recordLit()
		} else {
			x = p.ident()
		}
		if !lhs {
			// p.resolve(x)
		}
		return x
	case scan.Int, scan.Float, scan.String:
		x = &ast.BasicLit{Value: p.tok}
		p.next()
		return x
	case scan.Lparen:
		lparen := p.expect(scan.Lparen)
		// p.exprLev++
		x = p.rhs()
		// p.exprLev--
		rparen := p.expect(scan.Rparen)
		return &ast.ParenExpr{Lparen: lparen, X: x, Rparen: rparen}
	case scan.Fun:
		return p.funLit()
	}
	tok := p.tok
	p.error(tok, "expected operand")
	return &ast.BadExpr{From: tok, To: p.tok}
}

// Index = [ "[" ] "[" Expression "]" [ "]" ] .
// Slice = "[" [ Expression ] ":" [ Expression ] "]" .
func (p *parser) indexOrSlice(x ast.Expr) ast.Expr {
	lbrackOut := p.expect(scan.Lbrack)
	var lbrackIn scan.Token
	backwards := false
	slice := false
	if p.tok.Type == scan.Lbrack {
		lbrackIn = p.tok
		p.next()
		backwards = true
	}
	// p.exprLev++
	var index [2]ast.Expr
	var colon scan.Token
	if p.tok.Type != scan.Colon {
		index[0] = p.rhs()
	}
	if p.tok.Type == scan.Colon {
		if backwards {
			// error
			p.error(p.tok, "cannot slice a backwards index")
			return &ast.BadExpr{From: p.tok, To: p.tok}
		} else {
			slice = true
			colon = p.tok
			p.next()
			if p.tok.Type != scan.Rbrack && p.tok.Type != scan.EOF {
				index[1] = p.rhs()
			}
		}
	}
	// p.exprLev--
	rbrackIn := p.expect(scan.Rbrack)
	var rbrackOut scan.Token
	if backwards {
		rbrackOut = p.expect(scan.Rbrack)
	}
	if slice {
		return &ast.SliceExpr{
			X:      x,
			Lbrack: lbrackOut,
			Low:    index[0],
			Colon:  colon,
			High:   index[1],
			Rbrack: rbrackIn,
		}
	}
	return &ast.IndexExpr{
		X:         x,
		LbrackOut: lbrackOut,
		LbrackIn:  lbrackIn,
		Index:     index[0],
		Backwards: backwards,
		RbrackIn:  rbrackIn,
		RbrackOut: rbrackOut,
	}
}

// Selector = [ "." ] "." identifier .
func (p *parser) selector(x ast.Expr) ast.Expr {
	sel := p.ident()
	return &ast.SelectorExpr{X: x, Sel: sel}
}

func isLiteralType(x ast.Expr) bool {
	switch t := x.(type) {
	case *ast.BadExpr:
	case *ast.Ident:
	case *ast.SelectorExpr:
		_, isIdent := t.X.(*ast.Ident)
		return isIdent
	case *ast.ArrayLit:
	case *ast.RecordLit:
	default:
		return false
	}
	return true
}

// PrimaryExpr =
//     Operand |
//     Conversion |
//     PrimaryExpr Selector |
//     PrimaryExpr Index |
//     PrimaryExpr Slice |
//     PrimaryExpr Arguments .
func (p *parser) primaryExpr(lhs bool) ast.Expr {
	x := p.operand(lhs)
L:
	for {
		switch p.tok.Type {
		case scan.Period:
			p.next()
			if lhs {
				// p.resolve(x)
			}
			switch p.tok.Type {
			case scan.Ident:
				x = p.selector(x)
			default:
				tok := p.tok
				p.error(tok, "expected selector")
				p.next()
				tok.Lit = "_"
				sel := &ast.Ident{Name: tok}
				x = &ast.SelectorExpr{X: x, Sel: sel}
			}
		case scan.Lbrack:
			if lhs {
				// p.resolve(x)
			}
			x = p.indexOrSlice(x)
		case scan.Lparen:
			if lhs {
				// p.resolve(x)
			}
			x = p.callOrConversion(x)
		case scan.Lbrace:
			if isLiteralType(x) { // && (p.exprLev >= 0 || !isTypeName(x))
				if lhs {
					// p.resolve(x)
				}
				x = p.literalValue(x)
			} else {
				break L
			}
		default:
			break L
		}
		lhs = false
	}
	return x
}

func (p *parser) value(keyOk bool) ast.Expr {
	if p.tok.Type == scan.Lbrace {
		return p.literalValue(nil)
	}
	// possibly resolve key/field names
	return p.expr(keyOk)
}

// KeyedElement = [ Element ":" ] Element .
// Element = Expression .
func (p *parser) element() ast.Expr {
	x := p.value(true)
	if p.tok.Type == scan.Colon {
		colon := p.tok
		p.next()
		x = &ast.KeyValueExpr{Key: x, Colon: colon, Value: p.value(false)}
	}
	return x
}

// AssocLit = "{" [ ElementList [ "," ] ] "}" .
func (p *parser) literalValue(typ ast.Expr) ast.Expr {
	lbrace := p.expect(scan.Lbrace)
	var elts []ast.Expr
	// p.exprLev++
	if p.tok.Type != scan.Rbrace {
		for p.tok.Type != scan.Rbrace && p.tok.Type != scan.EOF {
			elts = append(elts, p.element())
			if !p.atComma("composite literal", scan.Rbrace) {
				break
			}
			p.next()
		}
	}
	// p.exprLev--
	rbrace := p.expectClosing(scan.Rbrace, "composite literal")
	return &ast.CompositeLit{Type: typ, Lbrace: lbrace, Elts: elts, Rbrace: rbrace}
}

func (p *parser) atComma(context string, follow scan.Type) bool {
	if p.tok.Type == scan.Comma {
		return true
	}
	if p.tok.Type != follow {
		msg := "missing ','"
		if p.tok.Type == scan.Semicolon && p.tok.Lit == "\n" {
			msg += " before newline"
		}
		p.error(p.tok, msg+" in "+context)
		return true
	}
	return false
}

func (p *parser) expectClosing(typ scan.Type, context string) scan.Token {
	if p.tok.Type != typ && p.tok.Type == scan.Semicolon && p.tok.Lit == "\n" {
		p.error(p.tok, "missing ',' before newline "+context)
		p.next()
	}
	return p.expect(typ)
}

// PrimaryExpr Arguments
// Arguments = "(" [ ExpressionList [ "..." ] [ "," ] ] ")" .
// Conversion = TypeName "(" Expression [ "," ] ")" .
// TypeName = identifier .
func (p *parser) callOrConversion(fn ast.Expr) *ast.CallExpr {
	lparen := p.expect(scan.Lparen)
	// p.exprLev++
	var list []ast.Expr
	var ellipsis scan.Token
	for p.tok.Type != scan.Rparen && p.tok.Type != scan.EOF {
		list = append(list, p.rhs())
		if p.tok.Type == scan.Ellipsis {
			ellipsis = p.tok
			p.next()
		}
		if !p.atComma("argument list", scan.Rparen) {
			break
		}
		p.next()
	}
	// p.exprLev--
	rparen := p.expectClosing(scan.Rparen, "argument list")
	return &ast.CallExpr{Fun: fn, Lparen: lparen, Args: list, Ellipsis: ellipsis, Rparen: rparen}
}

func (p *parser) declare(decl, data interface{}, scope *ast.Scope, kind ast.ObjKind, idents ...*ast.Ident) {
	for _, ident := range idents {
		obj := ast.NewObj(kind, ident.Name.Lit)
		obj.Decl = decl
		obj.Data = data
		if ident.Name.Lit != "_" {
			if alt := scope.Insert(obj); alt != nil {
				var prevDecl string
				if pos := alt.Tok(); pos.Line != 0 && pos.Offset != 0 {
					prevDecl = fmt.Sprintf("\n\tprevious declaration at %v", pos)
				}
				p.error(ident.Name, fmt.Sprintf("%s redeclared in this block%s", ident.Name.Lit, prevDecl))
			}
		}
	}
}

// StatementList = { Statement ";" } .
func (p *parser) stmtList() (list []ast.Stmt) {
	for p.tok.Type != scan.Case && p.tok.Type != scan.Default && p.tok.Type != scan.Rbrace && p.tok.Type != scan.EOF {
		list = append(list, p.stmt())
	}
	return
}

// SimpleStmt = EmptyStmt | ExpressionStmt | IncDecStmt | Assignment .
func (p *parser) simpleStmt(mode int) (ast.Stmt, bool) {
	xpos := p.tok
	x := p.lhsList()
	switch p.tok.Type {
	case
		scan.In, scan.Assign, scan.AddAssign, scan.SubAssign,
		scan.MulAssign, scan.QuoAssign, scan.RemAssign,
		scan.AndAssign, scan.OrAssign, scan.XorAssign,
		scan.ShlAssign, scan.ShrAssign, scan.AndNotAssign:
		tok := p.tok
		p.next()
		var y []ast.Expr
		isIn := false
		if mode == inOk && p.tok.Type == scan.In {
			tok := p.tok
			p.next()
			y = []ast.Expr{&ast.UnaryExpr{Op: tok, X: p.rhs()}}
			isIn = true
		} else {
			y = p.rhsList()
		}
		// we don't know if assignments are declarations yet
		as := &ast.AssignStmt{Lhs: x, Tok: tok, Rhs: y}
		return as, isIn
	}
	if len(x) > 1 {
		p.error(xpos, "1 expression")
	}
	switch p.tok.Type {
	case scan.Colon:
		colon := p.tok
		p.next()
		if label, isIdent := x[0].(*ast.Ident); mode == labelOk && isIdent {
			stmt := &ast.LabeledStmt{Label: label, Colon: colon, Stmt: p.stmt()}
			p.declare(stmt, nil, p.topScope, ast.Lbl, label)
			return stmt, false
		}
		p.error(colon, "illegal label declaration")
		return &ast.BadStmt{From: xpos, To: colon}, false
	case scan.Inc, scan.Dec:
		s := &ast.IncDecStmt{X: x[0], Tok: p.tok}
		p.next()
		return s, false
	}
	return &ast.ExprStmt{X: x[0]}, false
}

// ReturnStmt = "return" [ ExpressionList ] .
func (p *parser) returnStmt() *ast.ReturnStmt {
	tok := p.expect(scan.Return)
	var x []ast.Expr
	if p.tok.Type != scan.Semicolon && p.tok.Type != scan.Rbrace {
		x = p.rhsList()
	}
	p.expectSemi()
	return &ast.ReturnStmt{Return: tok, Results: x}
}

// BreakStmt = "break" [ Label ] .
// ContinueStmt = "continue" [ Label ] .
func (p *parser) branchStmt(keyword scan.Type) *ast.BranchStmt {
	tok := p.expect(keyword)
	var label *ast.Ident
	if tok.Type == scan.Ident {
		label = p.ident()
		// should we add to unresolved targets w/ targetstack?
	}
	p.expectSemi()
	return &ast.BranchStmt{Tok: tok, Label: label}
}

// Block = "{" StatementList "}" .
func (p *parser) blockStmt() *ast.BlockStmt {
	lbrace := p.expect(scan.Lbrace)
	p.openScope()
	list := p.stmtList()
	p.closeScope()
	rbrace := p.expect(scan.Rbrace)
	return &ast.BlockStmt{Lbrace: lbrace, List: list, Rbrace: rbrace}
}

func (p *parser) ifHeader() (init ast.Stmt, cond ast.Expr) {
	if p.tok.Type == scan.Lbrace {
		p.error(p.tok, "missing condition in if statement")
		cond = &ast.BadExpr{From: p.tok, To: p.tok}
		return
	}
	var cbeg, cend scan.Token
	// should we worry about preserving exprLev?
	if p.tok.Type != scan.Semicolon {
		if p.tok.Type == scan.Var {
			p.next()
			p.error(p.tok, "var declaration not allowed in 'IF' initializer")
		}
		cbeg = p.tok
		init, _ = p.simpleStmt(basic)
		cend = p.tok
	}
	var condStmt ast.Stmt
	var semi scan.Token
	if p.tok.Type != scan.Lbrace {
		if p.tok.Type == scan.Semicolon {
			semi = p.tok
			p.next()
		} else {
			p.expect(scan.Semicolon)
		}
		if p.tok.Type != scan.Lbrace {
			cbeg = p.tok
			condStmt, _ = p.simpleStmt(basic)
			cend = p.tok
		}
	} else {
		condStmt = init
		init = nil
	}
	if condStmt != nil {
		cond = p.makeExpr(condStmt, cbeg, cend, "boolean expression")
	} else if semi.Lit == "\n" {
		p.error(semi, "unexpected newline, expecting { after if clause")
	} else {
		p.error(semi, "missing condition in if statement")
	}
	if cond == nil {
		cond = &ast.BadExpr{From: p.tok, To: p.tok}
	}
	return
}

// IfStmt = "if" [ SimpleStmt ";" ] Expression Block [ "else" ( IfStmt | Block ) ] .
func (p *parser) ifStmt() *ast.IfStmt {
	tok := p.expect(scan.If)
	p.openScope()
	defer p.closeScope()
	init, cond := p.ifHeader()
	body := p.blockStmt()
	var else_ ast.Stmt
	if p.tok.Type == scan.Else {
		p.next()
		switch p.tok.Type {
		case scan.If:
			else_ = p.ifStmt()
		case scan.Lbrace:
			else_ = p.blockStmt()
			p.expectSemi()
		default:
			p.error(p.tok, "if statement or block")
			else_ = &ast.BadStmt{From: p.tok, To: p.tok}
		}
	} else {
		p.expectSemi()
	}
	return &ast.IfStmt{If: tok, Init: init, Cond: cond, Body: body, Else: else_}
}

// CaseClause = SwitchCase ":" StatementList .
// SwitchCase = "case" ExpressionList | "default" .
func (p *parser) caseClause() *ast.CaseClause {
	tok := p.tok
	var list []ast.Expr
	if p.tok.Type == scan.Case {
		p.next()
		list = p.rhsList()
	} else {
		p.expect(scan.Default)
	}
	colon := p.expect(scan.Colon)
	p.openScope()
	body := p.stmtList()
	p.closeScope()
	return &ast.CaseClause{Case: tok, List: list, Colon: colon, Body: body}
}

// SwitchStmt = ExprSwitchStmt .
// ExprSwitchStmt = "switch" [ SimpleStmt ";" ] [ Expression ] "{" { CaseClause } "}" .
func (p *parser) switchStmt() *ast.SwitchStmt {
	tok := p.expect(scan.Switch)
	p.openScope()
	defer p.closeScope()
	var s ast.Stmt
	var e ast.Expr
	if p.tok.Type != scan.Lbrace {
		// exprLev?
		if p.tok.Type != scan.Semicolon {
			s, _ = p.simpleStmt(basic)
		}
		if p.tok.Type == scan.Semicolon {
			p.next()
			if p.tok.Type != scan.Lbrace {
				// false?
				e = p.expr(false)
			}
		}
	}
	lbrace := p.expect(scan.Lbrace)
	var list []ast.Stmt
	for p.tok.Type == scan.Case || p.tok.Type == scan.Default {
		list = append(list, p.caseClause())
	}
	rbrace := p.expect(scan.Rbrace)
	p.expectSemi()
	body := &ast.BlockStmt{Lbrace: lbrace, List: list, Rbrace: rbrace}
	return &ast.SwitchStmt{Switch: tok, Init: s, Tag: e, Body: body}
}

func (p *parser) makeExpr(s ast.Stmt, beg, end scan.Token, want string) ast.Expr {
	if s == nil {
		return nil
	}
	if es, isExpr := s.(*ast.ExprStmt); isExpr {
		return es.X
	}
	found := "simple statement"
	if _, isAss := s.(*ast.AssignStmt); isAss {
		found = "assignment"
	}
	p.error(beg, fmt.Sprintf("expected %s, found %s (missing parentheses around composite literal?)", want, found))
	return &ast.BadExpr{From: beg, To: end}
}

// ForStmt = "for" [ Condition | ForClause | InClause ] Block .
// Condition = Expression .
// ForClause = [ InitStmt ] ";" [ Condition ] ";" [ PostStmt ] .
// InitStmt = SimpleStmt .
// PostStmt = SimpleStmt .
// InClause = IdentifierList "in" Expression .
func (p *parser) forStmt() ast.Stmt {
	tok := p.expect(scan.For)
	p.openScope()
	defer p.closeScope()

	var s1, s2, s3 ast.Stmt
	var isIn bool
	var s2beg, s2end scan.Token
	if p.tok.Type != scan.Lbrace {
		// exprLev?
		if p.tok.Type != scan.Semicolon {
			s2beg = p.tok
			if p.tok.Type == scan.In {
				// "for in x"
				tok := p.tok
				p.next()
				y := []ast.Expr{&ast.UnaryExpr{Op: tok, X: p.rhs()}}
				s2 = &ast.AssignStmt{Rhs: y}
				isIn = true
			} else {
				s2, isIn = p.simpleStmt(inOk)
			}
			s2end = p.tok
		}
		if !isIn && p.tok.Type == scan.Semicolon {
			p.next()
			s1 = s2
			s2 = nil
			s2beg, s2end = scan.Token{}, scan.Token{}
			if p.tok.Type != scan.Semicolon {
				s2beg = p.tok
				s2, _ = p.simpleStmt(basic)
				s2end = p.tok
			}
			p.expectSemi()
			if p.tok.Type != scan.Lbrace {
				s3, _ = p.simpleStmt(basic)
			}
		}
	}
	body := p.blockStmt()
	endb := p.tok
	p.expectSemi()
	if isIn {
		ltok := p.tok
		as := s2.(*ast.AssignStmt)
		var index, key, value ast.Expr
		switch len(as.Lhs) {
		case 0:
		case 1:
			key = as.Lhs[0]
		case 2:
			key, value = as.Lhs[0], as.Lhs[1]
		case 3: // necessary?
			index, key, value = as.Lhs[0], as.Lhs[1], as.Lhs[2]
		default:
			p.error(ltok, "at most 2 expressions")
			return &ast.BadStmt{From: tok, To: endb}
		}
		x := as.Rhs[0].(*ast.UnaryExpr).X
		return &ast.InStmt{
			For:   tok,
			Index: index,
			Key:   key,
			Value: value,
			Tok:   as.Tok,
			X:     x,
			Body:  body,
		}
	}
	return &ast.ForStmt{
		For:  tok,
		Init: s1,
		Cond: p.makeExpr(s2, s2beg, s2end, "boolean or in expression"),
		Post: s3,
		Body: body,
	}
}

// Statement =
//     Declaration | LabeledStmt | SimpleStmt | ReturnStmt |
//     BreakStmt | ContinueStmt | Block |
//     IfStmt | SwitchStmt | ForStmt .
func (p *parser) stmt() (s ast.Stmt) {
	switch p.tok.Type {
	case scan.Var:
		s = &ast.DeclStmt{Decl: p.genDecl(p.tok.Type, p.valueSpec)}
	case
		scan.Ident, scan.Int, scan.Float, scan.String, scan.Fun, scan.Lparen,
		scan.Add, scan.Sub, scan.Mul, scan.And, scan.Xor, scan.Not:
		s, _ = p.simpleStmt(labelOk)
		if _, isLabeledStmt := s.(*ast.LabeledStmt); !isLabeledStmt {
			p.expectSemi()
		}
	case scan.Return:
		s = p.returnStmt()
	case scan.Break, scan.Continue:
		s = p.branchStmt(p.tok.Type)
	case scan.Lbrace:
		s = p.blockStmt()
		p.expectSemi()
	case scan.If:
		s = p.ifStmt()
	case scan.Switch:
		s = p.switchStmt()
	case scan.For:
		s = p.forStmt()
	case scan.Semicolon:
		s = &ast.EmptyStmt{Semicolon: p.tok, Implicit: p.tok.Lit != ";"}
		p.next()
	case scan.Rbrace:
		s = &ast.EmptyStmt{Semicolon: p.tok, Implicit: true}
	default:
		t1 := p.tok
		p.error(p.tok, "statement")
		p.next()
		s = &ast.BadStmt{From: t1, To: p.tok}
	}
	return
}

func errd(es []Error) error {
	if len(es) == 0 {
		return nil
	}
	if len(es) == 1 {
		return es[0]
	}
	var s string
	for i := 0; i < len(es)-1; i++ {
		s += es[i].Error() + "\n"
	}
	s += es[len(es)-1].Error()
	return errors.New(s)
}

func File(f *ast.File) error {
	// SourceFile = [ PackageClause ";" ] { UseDecl ";" } StatementList .
	p := new(parser)
	p.sc = scan.New(bufio.NewReader(f.Src))
	p.next()
	if p.tok.Type == scan.Pkg {
		f.Package.Package = p.tok
		p.next()
		clause := p.ident()
		f.Package.Name = clause.Name.Lit
		p.expectSemi()
	}
	p.openScope()
	for p.tok.Type == scan.Use {
		f.Decls = append(f.Decls, p.genDecl(scan.Use, p.useSpec))
	}
	for p.tok.Type != scan.EOF {
		f.Stmts = append(f.Stmts, p.stmt())
	}
	p.closeScope()
	i := 0
	for _, id := range p.unresolved {
		id.Obj = p.pkgScope.Lookup(id.Name.Lit)
		if id.Obj == nil {
			p.unresolved[i] = id
			i++
		}
	}
	f.Scope = p.pkgScope
	f.Unresolved = p.unresolved
	return errd(p.errors)
}

// func Package(p *ast.Package) {

// }
