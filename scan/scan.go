package scan

import (
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type TokenScanner interface {
	Scan() Token
}

type Scanner struct {
	input  io.ByteReader
	buf    []byte // buffer to slice tokens
	pos    int    // current byte position in buffer
	start  int    // start position of this token in buffer
	line   int    // line number in input (1-indexed)
	width  int    // width of the last rune read from input
	offset int    // byte offset into input
	lo1    int    // previous byte offset at the beginning of the line
	lo2    int    // current byte offset at the beginning of the line
	curr   rune   // recently read rune
	bk     bool   // if the last pos update was a backup
	tok    *Token
	prev   *Token
}

const eof = -1

func (s *Scanner) next() rune {
	var (
		r    rune
		size int
	)
	if s.pos == len(s.buf) {
		b, err := s.input.ReadByte()
		if err == nil {
			s.buf = append(s.buf, b)
		}

		b, err = s.input.ReadByte()
		if err == nil {
			s.buf = append(s.buf, b)
		}

		b, err = s.input.ReadByte()
		if err == nil {
			s.buf = append(s.buf, b)
		}

		b, err = s.input.ReadByte()
		if err == nil {
			s.buf = append(s.buf, b)
		}
	}
	r, size = utf8.DecodeRune(s.buf[s.pos:])
	if size == 0 {
		r = eof
	}
	if s.curr == '\n' {
		s.line++
		s.lo1 = s.lo2
		s.lo2 = s.offset
	}
	s.width = size
	s.pos += s.width
	s.offset += s.width
	s.curr = r
	s.bk = false
	return r
}

func (s *Scanner) peek() rune {
	r := s.next()
	s.backup()
	return r
}

func (s *Scanner) backup() {
	if s.curr == '\n' && s.bk {
		s.line--
		s.lo2 = s.lo1
	}
	s.pos -= s.width
	s.offset -= s.width
	s.curr, _ = utf8.DecodeRune(s.buf[:s.pos])
	s.bk = true
}

func (s *Scanner) ignore() {
	s.start = s.pos
}

func (s *Scanner) emitType(t Type) {
	ts := string(s.buf[s.start:s.pos])
	l := len(s.buf[s.start:s.pos])
	s.tok = &Token{
		Type:   t,
		Offset: s.offset - l,
		Line:   s.line,
		Column: s.offset - l - s.lo2,
		Lit:    ts,
	}
	s.buf = s.buf[s.pos:]
	s.start = 0
	s.pos = 0
	s.width = 0
}

func (s *Scanner) emit(t Type, ts string) {
	l := len(s.buf[s.start:s.pos])
	s.tok = &Token{
		Type:   t,
		Offset: s.offset - l,
		Line:   s.line,
		Column: s.offset - l - s.lo2,
		Lit:    ts,
	}
	s.buf = s.buf[s.pos:]
	s.start = 0
	s.pos = 0
	s.width = 0
}

func (s *Scanner) errorf(format string, args ...interface{}) {
	s.emit(Illegal, fmt.Sprintf(format, args...))
}

func (s *Scanner) accept(valid string) bool {
	if strings.IndexRune(valid, s.next()) >= 0 {
		return true
	}
	s.backup()
	return false
}

func (s *Scanner) acceptRun(valid string) {
	for strings.IndexRune(valid, s.next()) >= 0 {
	}
	s.backup()
}

func New(r io.ByteReader) *Scanner {
	s := &Scanner{
		input: r,
		buf:   make([]byte, 0, 1024),
		line:  1,
	}
	return s
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDecimalDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func isOctalDigit(ch rune) bool {
	return '0' <= ch && ch <= '7'
}

func isHexDigit(ch rune) bool {
	return '0' <= ch && ch <= '9' || 'A' <= ch && ch <= 'F' || 'a' <= ch && ch <= 'f'
}

func print(r rune) {
	fmt.Printf("%c\n", r)
}

// assumes the first letter has already been consumed
func (s *Scanner) identifier() {
	ch := s.next()
	for isLetter(ch) || unicode.IsDigit(ch) {
		ch = s.next()
	}
	s.backup()

	ts := string(s.buf[s.start:s.pos])
	ttype := Lookup(ts)
	s.emit(ttype, ts)
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= ch && ch <= 'f':
		return int(ch - 'a' + 10)
	case 'A' <= ch && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 16 // larger than any legal digit val
}

func (s *Scanner) scanMantissa(base int) {
	ch := s.next()
	for digitVal(ch) < base {
		ch = s.next()
	}
	s.backup()
}

// '.', 'e', 'E' passed in
func (s *Scanner) floating(ch rune) {
	if ch == '.' {
		s.scanMantissa(10)
	}
	if ch == 'e' || ch == 'E' {
		s.accept("-+")
		if ch = s.next(); digitVal(ch) < 10 {
			s.scanMantissa(10)
		} else {
			s.errorf("%v illegal floating-point exponent", s.line)
			return
		}
	}
	s.emitType(Float)
}

// first decimal digit is passed in
func (s *Scanner) number(ch rune) {
	// must be octal, hex or float
	if ch == '0' {
		if s.accept("xX") {
			// must be a hex
			pos := s.pos
			s.scanMantissa(16)
			if s.pos == pos {
				// only scanned "0x" or "0X"
				s.errorf("%v illegal hexadecimal number", s.line)
				return
			}
		} else {
			// octal int or float
			mustBeFloat := false
			s.scanMantissa(8)
			ch = s.next()
			if ch == '8' || ch == '9' {
				// float or illegal octal int
				mustBeFloat = true
				s.scanMantissa(10)
			}
			if ch == '.' || ch == 'e' || ch == 'E' {
				s.floating(rune(s.buf[s.pos-1]))
				return
			}
			if mustBeFloat {
				s.errorf("%v illegal octal number", s.line)
				return
			}
			s.backup()
		}
	} else {
		// decimal int or float
		s.scanMantissa(10)
		if s.accept(".eE") {
			s.floating(rune(s.buf[s.pos-1]))
			return
		}
	}
	s.emitType(Int)
}

// Assumes the initial '\' has already been read.
func (s *Scanner) scanEscape(quote rune) error {
	var n int
	var base, max uint32

	ch := s.next()
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		return nil
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 255
	case 'x':
		ch = s.next()
		n, base, max = 2, 16, 255
	case 'u':
		ch = s.next()
		n, base, max = 4, 16, unicode.MaxRune
	case 'U':
		ch = s.next()
		n, base, max = 8, 16, unicode.MaxRune
	default:
		msg := "unknown escape sequence"
		if ch < 0 {
			msg = "escape sequence not terminated"
		}
		return fmt.Errorf("%v %s", s.line, msg)
	}

	var x uint32
	for {
		d := uint32(digitVal(ch))
		if d >= base {
			msg := fmt.Sprintf("illegal character %#U in escape sequence", ch)
			if ch < 0 {
				msg = "escape sequence not terminated"
			}
			return fmt.Errorf("%v %s", s.line, msg)
		}
		x = x*base + d
		n--
		if n <= 0 {
			break
		}
		ch = s.next()
	}

	if x > max || 0xD800 <= x && x < 0xE000 {
		return fmt.Errorf("%v escape sequence is invalid Unicode code point", s.line)
	}
	return nil
}

func (s *Scanner) scanString() {
	for {
		ch := s.next()
		if ch == '\n' || ch < 0 {
			s.errorf("%v string literal not terminated", s.line)
			return
		}
		if ch == '\'' {
			break
		}
		if ch == '\\' {
			if err := s.scanEscape('\''); err != nil {
				s.errorf(err.Error())
				return
			}
		}
	}
	s.emitType(String)
}

func (s *Scanner) scanRawString() {
	hasCR := false
	nl := 0
	lo := -1
	for {
		ch := s.next()
		if lo < 0 {
			lo = s.lo2
		}
		if ch < 0 {
			s.errorf("%v raw string literal not terminated", s.line)
			return
		}
		if ch == '`' {
			break
		}
		if ch == '\r' {
			hasCR = true
		}
		if ch == '\n' {
			nl++
		}
	}

	tb := s.buf[s.start:s.pos]
	if hasCR {
		tb = stripCR(tb)
	}
	// emit with line offset
	s.tok = &Token{
		Type:   String,
		Offset: s.offset - len(tb),
		Line:   s.line - nl,
		Column: lo,
		Lit:    string(tb),
	}
	s.buf = s.buf[s.pos:]
	s.start = 0
	s.pos = 0
	s.width = 0
}

func stripCR(b []byte) []byte {
	c := make([]byte, len(b))
	i := 0
	for _, ch := range b {
		if ch != '\r' {
			c[i] = ch
			i++
		}
	}
	return c[:i]
}

func (s *Scanner) scanComment(ch rune) {
	hasCR := false
	if ch == '/' {
		//-style comment
		// scan until newline or eof
		for {
			ch = s.next()
			if ch == '\n' || ch < 0 {
				break
			}
			if ch == '\r' {
				// TODO: strip carriage returns by ignoring character instead of 2nd pass?
				hasCR = true
			}
		}
	} else if ch == '*' {
		/*-style comment */
		// scan until ('*' and '/')
		ch = s.next()
		for {
			if ch < 0 {
				s.errorf("%v comment not terminated", s.line)
				return
			}
			if ch == '\r' {
				hasCR = true
			}
			c := s.next()
			if ch == '*' && c == '/' {
				break
			}
			ch = c
		}
	}
	tb := s.buf[s.start:s.pos]
	if hasCR {
		tb = stripCR(tb)
	}
	s.emitType(Comment)
}

func (s *Scanner) switch2(t1, t2 Type) {
	c := s.next()
	if c == '=' {
		s.emitType(t2)
	} else {
		s.backup()
		s.emitType(t1)
	}
}

func (s *Scanner) switch3(t1, t2 Type, ch rune, t3 Type) {
	c := s.next()
	if c == '=' {
		s.emitType(t2)
	} else if c == ch {
		s.emitType(t3)
	} else {
		s.backup()
		s.emitType(t1)
	}
}

func (s *Scanner) insertSemi() bool {
	if s.prev != nil {
		switch s.prev.Type {
		case Ident, Int, Float, String, Break, Continue, Return, Inc, Dec, Rparen, Rbrack, Rbrace:
			s.emitType(Semicolon)
			return true
		}
	}
	return false
}

func (s *Scanner) Scan() Token {
	for s.tok == nil {
		ch := s.next()
		switch {
		case isLetter(ch):
			s.identifier()
		case isDecimalDigit(ch):
			s.number(ch)
		default:
			switch ch {
			case -1:
				if !s.insertSemi() {
					s.emitType(EOF)
				}
			case ' ', '\t', '\r':
				s.ignore()
			case '\n':
				s.insertSemi()
				s.ignore()
			case '\'':
				s.scanString()
			case '`':
				s.scanRawString()
			case ':':
				s.emitType(Colon)
			case '.':
				c := s.peek()
				if isDecimalDigit(c) {
					s.floating(ch)
				} else if c == '.' {
					s.next()
					c = s.next()
					if c == '.' {
						s.emitType(Ellipsis)
					}
				} else {
					s.emitType(Period)
				}
			case ',':
				s.emitType(Comma)
			case ';':
				s.emitType(Semicolon)
			case '(':
				s.emitType(Lparen)
			case ')':
				s.emitType(Rparen)
			case '[':
				s.emitType(Lbrack)
			case ']':
				s.emitType(Rbrack)
			case '{':
				s.emitType(Lbrace)
			case '}':
				s.emitType(Rbrace)
			case '+':
				s.switch3(Add, AddAssign, '+', Inc)
			case '-':
				s.switch3(Sub, SubAssign, '-', Dec)
			case '*':
				s.switch2(Mul, MulAssign)
			case '/':
				c := s.next()
				if c == '/' || c == '*' {
					// read until newline or eof. strip carriage returns
					s.scanComment(c)
				} else if c == '=' {
					s.emitType(QuoAssign)
				} else {
					s.backup()
					s.emitType(Quo)
				}
			case '%':
				s.switch2(Rem, RemAssign)
			case '^':
				s.switch2(Xor, XorAssign)
			case '<':
				c := s.next()
				if c == '=' {
					s.emitType(Leq)
				} else if c == '<' {
					if s.next() == '=' {
						s.emitType(ShlAssign)
					} else {
						s.backup()
						s.emitType(Shl)
					}
				} else {
					s.backup()
					s.emitType(Lss)
				}
			case '>':
				c := s.next()
				if c == '=' {
					s.emitType(Geq)
				} else if c == '>' {
					if s.next() == '=' {
						s.emitType(ShrAssign)
					} else {
						s.backup()
						s.emitType(Shr)
					}
				} else {
					s.backup()
					s.emitType(Gtr)
				}
			case '=':
				s.switch2(Assign, Eql)
			case '!':
				s.switch2(Not, Neq)
			case '&':
				c := s.next()
				if c == '^' {
					if s.next() == '=' {
						s.emitType(AndNotAssign)
					} else {
						s.backup()
						s.emitType(AndNot)
					}
				} else if c == '&' {
					s.emitType(Land)
				} else if c == '=' {
					s.emitType(AndAssign)
				} else {
					s.backup()
					s.emitType(And)
				}
			case '|':
				s.switch3(Or, OrAssign, '|', Lor)
			default:
				s.errorf("illegal character %#U", ch)
			}
		}
	}
	t := *s.tok
	s.prev = s.tok
	s.tok = nil
	return t
}
