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
	Char := stateful.Rule{`Char`, `.`, nil}
	AssignOp := stateful.Rule{`AssignOp`, `::=|:=|\?=|!=|=`, stateful.Push("Expr")}
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
			ExpStart,
			{"nl", `\n+`, nil},
			{`ExpEnd`, `\)`, stateful.Pop()},
			{`ExpStr`, `'[^']*'|"[^"]*"`, nil},
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
			ExpStart,
			Char,
		},
		"Keyword": {
			stateful.Include("Common"),
			{`Nl`, `\n`, stateful.Push("Root")},
			stateful.Include("Expr"),
		},
		"RootBody": {
			stateful.Include("Base"),
			{`Nl`, `\n`, nil},
			{`Tab`, `\t`, stateful.Push("Expr")},
			ExpStart,
			{`Define`, "define", stateful.Push("RootBody")},
			{`Keyword`, KeywordPattern, stateful.Push("Keyword")},
			AssignOp,
			{`Char`, `.`, nil},
		},
		"Root": {
			stateful.Include("Common"),
			AssignOp,
			{`Colon`, `:`, stateful.Push("TargetDeps")},
			stateful.Include("RootBody"),
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
