package lexer

import (
	"github.com/alecthomas/participle/v2/lexer"
	"github.com/alecthomas/participle/v2/lexer/stateful"
	"io"
	"strings"
)

var _def *stateful.Definition

func init() {
	ExpStart := stateful.Rule{`ExpStart`, `\$\(`, stateful.Push("Exp")}
	ExpVar := stateful.Rule{`ExpVar`, `\$[\d]+|\$[\w-]+`, nil}
	Char := stateful.Rule{`Char`, `.|\n`, nil}
	AssignOp := stateful.Rule{`AssignOp`, `::=|:=|\?=|!=|\+=|=`, stateful.Push("Expr")}
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
			{`Comment`, `#[^\n]*\n`, nil},
			{`Escaped`, `\\.`, nil},
		},
		"Common": {
			stateful.Include("Base"),
			{"ws", `[ ]+`, nil},
		},
		"Exp": {
			stateful.Include("Base"),
			{`ExpEnd`, `\)`, stateful.Pop()},
			{`ExpStr`, `'[^']*'|"[^"]*"`, nil},
			ExpVar,
			ExpStart,
			Char,
		},
		"TargetDeps": {
			stateful.Include("Base"),
			{`TargetDepsEnd`, `\n`, stateful.Push("Root")},
			stateful.Include("Expr"),
		},
		"Expr": {
			stateful.Include("Base"),
			{`Nl`, `\n`, stateful.Pop()},
			ExpVar,
			ExpStart,
			Char,
		},
		"Define": {
			stateful.Include("Base"),
			{`Endef`, `endef`, stateful.Pop()},
			ExpVar,
			ExpStart,
			Char,
		},
		"Keyword": {
			stateful.Include("Common"),
			{`Nl`, `\n`, stateful.Push("Root")},
			stateful.Include("Expr"),
		},
		"Root": {
			stateful.Include("Common"),
			AssignOp,
			{`Colon`, `:`, stateful.Push("TargetDeps")},
			{`Nl`, `\n`, nil},
			{`Tab`, `\t`, stateful.Push("Expr")},
			ExpVar,
			ExpStart,
			{`Define`, "define", stateful.Push("Define")},
			{`Keyword`, KeywordPattern, stateful.Push("Keyword")},
			AssignOp,
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
