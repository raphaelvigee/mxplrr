package lexer

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/alecthomas/participle/v2/lexer/stateful"
	"io"
	"strings"
)

var _def *stateful.Definition

func init() {
	ExpStart := stateful.Rule{`ExpStart`, `\$[({]`, stateful.Push("Exp")}
	ExpVar := stateful.Rule{`ExpVar`, `\$[\d]+|\$[\w]`, nil}
	Char := stateful.Rule{`Char`, `.|\n`, nil}
	AssignOp := stateful.Rule{`AssignOp`, `::=|:=|\?=|!=|\+=|=`, nil}
	KeywordPattern := strings.Join([]string{
		"endif",
		"ifeq",
		"ifneq",
		"ifdef",
		"ifndef",
		"include",
		"define",
		"endef",
	}, "|")

	_def = stateful.Must(stateful.Rules{
		"Base": {
			{"line_continuation", `\\\n\s*`, nil},
			{`Comment`, `#[^\n]*`, nil},
			{`Escaped`, `\\.|[$]{2}`, nil},
		},
		"Common": {
			stateful.Include("Base"),
		},
		"Exp": {
			stateful.Include("Base"),
			{`ExpEnd`, `[)}]`, stateful.Pop()},
			{`ExpStr`, `'[^']*'|"[^"]*"`, nil},
			ExpVar,
			ExpStart,
			Char,
		},
		"Keyword": {
			stateful.Include("Common"),
			{`Nl`, `\n`, stateful.Push("Root")},
			stateful.Include("Root"),
		},
		"Root": {
			stateful.Include("Common"),
			AssignOp,
			{`Colon`, `:`, nil},
			{`Nl`, `\n`, nil},
			{`Tab`, `\t`, nil},
			ExpVar,
			ExpStart,
			{`Keyword`, KeywordPattern, stateful.Push("Keyword")},
			Char,
		},
	})
}

func Tokenize(r io.Reader) ([]Token, error) {
	lex, err := Def().Lex("", r)
	if err != nil {
		panic(err)
	}

	toks, err := lexer.ConsumeAll(lex)
	if err != nil {
		return nil, err
	}

	mytoks := make([]Token, len(toks))
	for i, t := range toks {
		mytoks[i] = Token(t)
	}

	return mytoks, nil
}

func Def() *stateful.Definition {
	return _def
}

func Symbols() map[string]rune {
	return Def().Symbols()
}

func Symbol(name string) rune {
	t := Symbols()[name]
	if t == 0 {
		panic("unknown symbol: " + name)
	}
	return t
}

var typeToName map[rune]string

func init() {
	typeToName = map[rune]string{}
	for s, k := range Symbols() {
		typeToName[k] = s
	}
}

func SymbolName(t rune) string {
	return typeToName[t]
}
