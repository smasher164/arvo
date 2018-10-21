package scan

import (
	"strings"
	"testing"
)

// TODO(akhil): Add WAY more (categories of) test cases.

var cases = []struct {
	input string
	want  []Token
}{
	{
		input: `a b c`,
		want: []Token{
			{Ident, 0, 1, 0, "a"},
			{Ident, 2, 1, 2, "b"},
			{Ident, 4, 1, 4, "c"},
			{Semicolon, 5, 1, 5, ""},
		},
	},

	{
		input: `fun x`,
		want: []Token{
			{Fun, 0, 1, 0, "fun"},
			{Ident, 4, 1, 4, "x"},
			{Semicolon, 5, 1, 5, ""},
		},
	},

	{
		input: `'a' '\t' '\xFF'`,
		want: []Token{
			{String, 0, 1, 0, `'a'`},
			{String, 4, 1, 4, `'\t'`},
			{String, 9, 1, 9, `'\xFF'`},
			{Semicolon, 15, 1, 15, ""},
		},
	},

	{
		input: "a\nb",
		want: []Token{
			{Ident, 0, 1, 0, "a"},
			{Semicolon, 1, 1, 1, "\n"},
			{Ident, 2, 2, 0, "b"},
			{Semicolon, 3, 2, 1, ""},
		},
	},

	{
		input: "`ab\ncd`",
		want: []Token{
			{String, 0, 1, 0, "`ab\ncd`"},
			{Semicolon, 7, 2, 3, ""},
		},
	},

	{
		input: "12345 123.45 123e45",
		want: []Token{
			{Int, 0, 1, 0, "12345"},
			{Float, 6, 1, 6, "123.45"},
			{Float, 13, 1, 13, "123e45"},
			{Semicolon, 19, 1, 19, ""},
		},
	},

	{
		input: `'abcd' '\t \n\''`,
		want: []Token{
			{String, 0, 1, 0, `'abcd'`},
			{String, 7, 1, 7, `'\t \n\''`},
			{Semicolon, 16, 1, 16, ""},
		},
	},
}

func TestScan(t *testing.T) {
	for i, tc := range cases {
		sc := New(strings.NewReader(tc.input))
		n := len(tc.want)
		j := 0
		for {
			tok := sc.Scan()
			if n == 0 && tok.Type != EOF {
				t.Errorf("case #%d, wanted %d tokens", i, n)
				break
			}
			if n > 0 && tok.Type == EOF {
				t.Errorf("case #%d, wanted %d tokens", i, n)
				break
			}
			if j == len(tc.want) {
				break
			}
			if tc.want[j] != tok {
				t.Errorf("case #%d, wanted: %v, got: %v", i, tc.want[j], tok)
				break
			}
			j++
			n--
		}
	}
}
