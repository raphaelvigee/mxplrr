package main

import (
	"fmt"
	"github.com/alecthomas/repr"
	"log"
	"makexplorer/lexer"
	"makexplorer/parser"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	p := os.Args[1]
	var err error
	if strings.HasSuffix(p, "/...") {
		p = strings.TrimSuffix(p, "/...")
		c := 0
		err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if info.Name() == "Makefile" || strings.HasSuffix(info.Name(), ".mk") {
				fmt.Println(path)
				c++
				return parse(path, false)
			}

			return nil
		})
		fmt.Printf("Found %v files\n", c)
	} else {
		err = parse(p, true)
	}

	if err != nil {
		log.Fatal(err)
	}
}

func parse(path string, print bool) error {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	tokens, err := lexer.Tokenize(f)
	if err != nil {
		return err
	}

	if print {
		PrintTokens(tokens)
		fmt.Println()
	}

	node, err := parser.ParseTokens(tokens)
	if print {
		repr.Println(node)
		fmt.Println()
	}

	if err != nil {
		return err
	}
	return nil
}

func PrintTokens(tokens []lexer.Token) {
	for _, t := range tokens {
		fmt.Println(t.StringAlign())
	}
}
