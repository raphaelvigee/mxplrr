package lexer

import (
	"fmt"
	plexer "github.com/alecthomas/participle/v2/lexer"
)

type Token plexer.Token

var EOF = plexer.EOF

var NilToken = Token{Type: EOF}

func (t Token) StringAlign() string {
	return fmt.Sprintf("%-15v %7v %q", SymbolName(t.Type), t.Pos, t.Value)
}

func (t Token) String() string {
	return fmt.Sprintf("%v %v %q", SymbolName(t.Type), t.Pos, t.Value)
}
