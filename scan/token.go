package scan

import "strconv"

type Type int

const (
	EOF Type = iota
	Illegal
	Comment

	// Identifiers and basic type literals
	// (these tokens stand for classes of literals)
	Ident  // add, true, false
	Int    // 12345
	Float  // 123.45
	String // 'a', `a`

	// Operators and delimiters
	Add // +
	Sub // -
	Mul // *
	Quo // /
	Rem // %

	And    // &
	Or     // |
	Xor    // ^
	Shl    // <<
	Shr    // >>
	AndNot // &^

	assignment_beg
	Assign    // =
	AddAssign // +=
	SubAssign // -=
	MulAssign // *=
	QuoAssign // /=
	RemAssign // %=

	AndAssign    // &=
	OrAssign     // |=
	XorAssign    // ^=
	ShlAssign    // <<=
	ShrAssign    // >>=
	AndNotAssign // &^=
	assignment_end

	Land // &&
	Lor  // ||
	Inc  // ++
	Dec  // --

	Eql // ==
	Lss // <
	Gtr // >
	Not // !

	Neq      // !=
	Leq      // <=
	Geq      // >=
	Ellipsis // ...

	Lparen // (
	Lbrack // [
	Lbrace // {
	Comma  // ,
	Period // .

	Rparen    // )
	Rbrack    // ]
	Rbrace    // }
	Semicolon // ;
	Colon     // :

	keyword_beg
	// keywords
	Switch
	Case
	Break
	Continue
	Default

	If
	Else

	Fun
	Return

	For
	In

	Var

	Use
	Pkg
	keyword_end
)

var ttypes = [...]string{
	EOF:     "EOF",
	Illegal: "Illegal",
	Comment: "Comment",
	Ident:   "Ident",
	Int:     "Int",
	Float:   "Float",
	String:  "String",

	Add: "+",
	Sub: "-",
	Mul: "*",
	Quo: "/",
	Rem: "%",

	And:    "&",
	Or:     "|",
	Xor:    "^",
	Shl:    "<<",
	Shr:    ">>",
	AndNot: "&^",

	Assign:    "=",
	AddAssign: "+=",
	SubAssign: "-=",
	MulAssign: "*=",
	QuoAssign: "/=",
	RemAssign: "%=",

	AndAssign:    "&=",
	OrAssign:     "|=",
	XorAssign:    "^=",
	ShlAssign:    "<<=",
	ShrAssign:    ">>=",
	AndNotAssign: "&^=",

	Land: "&&",
	Lor:  "||",
	Inc:  "++",
	Dec:  "--",

	Eql: "==",
	Lss: "<",
	Gtr: ">",
	Not: "!",

	Neq:      "!=",
	Leq:      "<=",
	Geq:      ">=",
	Ellipsis: "...",

	Lparen: "(",
	Lbrack: "[",
	Lbrace: "{",
	Comma:  ",",
	Period: ".",

	Rparen:    ")",
	Rbrack:    "]",
	Rbrace:    "}",
	Semicolon: ";",
	Colon:     ":",

	Switch:   "switch",
	Case:     "case",
	Break:    "break",
	Continue: "continue",
	Default:  "default",

	If:   "if",
	Else: "else",

	Fun:    "fun",
	Return: "return",

	For: "for",
	In:  "in",

	Var: "var",
	Use: "use",

	Pkg: "pkg",
}

var keywords map[string]Type

func init() {
	keywords = make(map[string]Type)
	for i := keyword_beg + 1; i < keyword_end; i++ {
		keywords[ttypes[i]] = i
	}
}

func (typ Type) String() string {
	s := ""
	if 0 <= typ && typ < Type(len(ttypes)) {
		s = ttypes[typ]
	}
	if s == "" {
		s = "type(" + strconv.Itoa(int(typ)) + ")"
	}
	return s
}

// A set of constants for precedence-based expression parsing.
// Non-operators have lowest precedence, followed by operators
// starting with precedence 1 up to unary operators. The highest
// precedence serves as "catch-all" precedence for selector,
// indexing, and other operator and delimiter tokens.
//
const (
	LowestPrec  = 0 // non-operators
	UnaryPrec   = 6
	HighestPrec = 7
)

// Precedence returns the operator precedence of the binary
// operator op. If op is not a binary operator, the result
// is LowestPrecedence.
//
func (op Type) Precedence() int {
	switch op {
	case Lor:
		return 1
	case Land:
		return 2
	case Eql, Neq, Lss, Leq, Gtr, Geq:
		return 3
	case Add, Sub, Or, Xor:
		return 4
	case Mul, Quo, Rem, Shl, Shr, And, AndNot:
		return 5
	}
	return LowestPrec
}

func (t Type) IsAssignment() bool {
	return t > assignment_beg && t < assignment_end
}

func Lookup(ident string) Type {
	if ttype, is_keyword := keywords[ident]; is_keyword {
		return ttype
	}
	return Ident
}

// should we separate token type and positional information?

type Token struct {
	Type   Type
	Offset int
	Line   int
	Column int
	Lit    string
}
