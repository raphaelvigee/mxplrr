package main

import (
	"github.com/alecthomas/participle/v2/experimental/codegen"
	"makexplorer/lexer"
	"os"
)

func main() {
	f, err := os.Create("lexer/gen.go")
	if err != nil {
		panic(err)
	}

	err = codegen.GenerateLexer(f, "lexer", lexer.Def())
	if err != nil {
		panic(err)
	}
}
