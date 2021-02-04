package cmd

import (
	"fmt"
	"github.com/alecthomas/repr"
	"github.com/spf13/cobra"
	"mxplrr/lexer"
	"mxplrr/parser"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	rootCmd.AddCommand(dumpCmd)
}

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := args[0]
		if strings.HasSuffix(p, "/...") {
			p = strings.TrimSuffix(p, "/...")
			c := 0
			err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
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
			return err
		} else {
			return parse(p, true)
		}
	},
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

	p := parser.NewParserTokens(tokens)

	node, err := p.Parse()
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
