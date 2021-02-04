package parser

import (
	"errors"
	"io"
	"mxplrr/lexer"
	"os"
	"strings"
)

var NlMatcher = lexer.NewMultiMatcher(lexer.NewMatcher("Char", "\n"), lexer.NewMatcher("Nl"))

func isEOF(t lexer.Token) bool {
	return lexer.NewMatcher("EOF").Is(t)
}

func NewParser(r io.Reader) (*Parser, error) {
	toks, err := lexer.Tokenize(r)
	if err != nil {
		return nil, err
	}

	return NewParserTokens(toks), nil
}

func NewParserString(s string) (*Parser, error) {
	return NewParser(strings.NewReader(s))
}

func ParseFile(filename string) (*File, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p, err := NewParser(f)
	if err != nil {
		return nil, err
	}

	n, err := p.Parse()
	if err != nil {
		return nil, err
	}

	return &File{
		Path:  filename,
		Nodes: n,
	}, nil
}

func NewParserTokens(tokens []lexer.Token) *Parser {
	return &Parser{
		tokens: tokens,
	}
}

type Parser struct {
	ParseVarBody bool
	tokens       []lexer.Token
	c            int
	lastComments []string
}

func (p *Parser) root(t lexer.Token) (outNode Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("root", rerr)
		}
	}()

	if lexer.NewMatcher("Comment").Is(t) {
		p.advance()
		p.lastComments = append(p.lastComments, strings.TrimSuffix(t.Value, "\n"))
		return p.root(p.peekn(0))
	}

	if lexer.NewMatcher("Char", "-", "+").Is(t) {
		m := p.advance() // Eat modifier

		n, err := p.root(p.peekn(0))
		if err != nil {
			return nil, err
		}

		return &Modifier{
			Modifier: m.Value,
			Node:     n,
		}, nil
	}

	defer func() {
		if outNode != nil && len(p.lastComments) > 0 {
			outNode.SetComments(p.lastComments)
			p.lastComments = nil
		}
	}()

	switch t.Type {
	case lexer.EOF:
		p.advance() // Eat EOF
		return nil, io.EOF
	case lexer.Symbol("Nl"):
		p.lastComments = nil
		p.advance() // Eat \n
		return p.root(p.peekn(0))
	case lexer.Symbol("Keyword"):
		p.advance()
		switch t.Value {
		case "include":
			return p.include()
		case "define":
			return p.define()
		case "ifeq", "ifneq":
			return p.ifeq(t)
		case "ifdef", "ifndef":
			return p.ifdef(t)
		case "endif", "endef":
			return nil, p.ut(t)
		}

		panic("keyword `" + t.Value + "` needs implementing")
	}

	exp, err := p.expr(false, lexer.NewMultiMatcher(
		NlMatcher,
		lexer.NewMatcher("Colon"),
		lexer.NewMatcher("AssignOp"),
	))
	if err != nil {
		return nil, err
	}

	opt := p.peekn(0)
	switch opt.Type {
	case lexer.Symbol("Colon"):
		p.advance() // Eat :
		return p.target(exp)
	}

	if exp != nil {
		switch opt.Type {
		case lexer.Symbol("Nl"):
			p.advance() // Eat \n

		case lexer.Symbol("AssignOp"):
			p.advance() // Eat op
			return p.varass(exp, opt)
		}

		return exp, nil
	}

	return nil, p.ut(t)
}

func (p *Parser) Parse() (Node, error) {
	return p.parse()
}

func (p *Parser) ParseExpr() (Node, error) {
	return p.expr(false, lexer.NewMatcher("EOF"))
}

func (p *Parser) parse() (Node, error) {
	nodes := make(Nodes, 0)

	for {
		t := p.peekn(0)
		n, err := p.root(t)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nodes, err
		}
		nodes = append(nodes, n)
	}

	if len(nodes) == 1 {
		return nodes[0], nil
	}

	return nodes, nil
}

func (p *Parser) include() (*Include, error) {
	p.eatall(lexer.NewMatcher("Char", " "))

	expr, err := p.expr(true, lexer.NewMatcher("Nl"))
	if err != nil {
		return nil, err
	}

	return &Include{
		Path: expr,
	}, nil
}

func (p *Parser) expr(eat bool, matcher lexer.Matcher) (_ Node, rerr error) {
	return p._expr(exprOptions{
		matcher:    matcher,
		eat:        eat,
		rawMatcher: matcher,
	})
}

type exprOptions struct {
	matcher lexer.Matcher
	eat     bool

	rawMatcher lexer.Matcher
	rawDrop    DropFunc
}

func (p *Parser) _expr(o exprOptions) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("expr", rerr)
		}
	}()

	expr := &Expr{}

	for {
		t := p.peekn(0)
		if stop := o.matcher.Is(t); stop {
			if o.eat {
				p.advance()
			}

			break
		}

		exp, err := p.exp()
		if err != nil {
			return nil, err
		}
		if exp != nil {
			expr.Parts = append(expr.Parts, exp)
			continue
		}

		raw, err := p.raw(func(t lexer.Token) (bool, bool) {
			if lexer.NewMultiMatcher(lexer.NewMatcher("ExpVar"), lexer.NewMatcher("ExpStart"), lexer.NewMatcher("ExpEnd")).Is(t) {
				return true, false
			}

			return o.rawMatcher.Is(t), false
		}, o.rawDrop)
		if err != nil {
			return nil, err
		}
		if raw != nil {
			expr.Parts = append(expr.Parts, raw)
			continue
		}

		break
	}

	switch len(expr.Parts) {
	case 0:
		return nil, nil
	case 1:
		return expr.Parts[0], nil
	}

	return expr, nil
}

func (p *Parser) exp() (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("exp", rerr)
		}
	}()

	t := p.peekn(0)
	if lexer.NewMatcher("ExpStart").Is(t) {
		p.advance() // Eat $(
		exp := &Exp{}

		for {
			t := p.peekn(0)
			if isEOF(t) {
				return nil, p.err("unexpected eof")
			}

			if lexer.NewMatcher("ExpEnd").Is(t) {
				p.advance() // Eat )
				return exp, nil
			}

			if lexer.NewMatcher("Char", ",").Is(t) {
				p.advance() // Eat ,
			}

			// Patsubst
			if len(exp.Parts) == 1 && lexer.NewMatcher("Char", ":").Is(t) {
				p.advance() // :

				pattern, err := p.expr(true, lexer.NewMatcher("Char", "="))
				if err != nil {
					return nil, err
				}
				if pattern == nil {
					pattern = &Raw{}
				}

				subst, err := p.expr(true, lexer.NewMatcher("ExpEnd", ")"))
				if err != nil {
					return nil, err
				}
				if subst == nil {
					subst = &Raw{}
				}

				return &PatSubst{
					Name:    exp.Parts[0],
					Pattern: pattern,
					Subst:   subst,
				}, nil
			}

			isFirst := len(exp.Parts) == 0
			sepMatcher := func() lexer.Matcher {
				if isFirst {
					return lexer.NewMultiMatcher(NlMatcher, lexer.NewMatcher("Char", " ", ":"))
				}

				return lexer.NewMatcher("Char", ",")
			}()

			part, err := p._expr(exprOptions{
				matcher:    sepMatcher,
				eat:        false,
				rawMatcher: sepMatcher,
			})
			if err != nil {
				return exp, err
			}

			p.eat(lexer.NewMultiMatcher(NlMatcher, lexer.NewMatcher("Char", " ")))

			if part == nil {
				if isFirst {
					return exp, p.ut(t)
				}

				part = &Raw{}
			}

			exp.Parts = append(exp.Parts, part)
		}
	} else if lexer.NewMatcher("ExpVar").Is(t) {
		p.advance() // Eat $...
		return &Exp{
			Parts: []Node{&Raw{Text: strings.TrimPrefix(t.Value, "$")}},
		}, nil
	}

	return nil, nil
}

type UntilFunc func(token lexer.Token) (bool, bool)
type DropFunc func(token lexer.Token) bool

func (p *Parser) raw(until UntilFunc, drop DropFunc) (_ *Raw, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("raw", rerr)
		}
	}()
	if drop == nil {
		drop = func(token lexer.Token) bool {
			return false
		}
	}

	acc := ""
	for {
		p.eat(lexer.NewMatcher("Comment"))

		t := p.peekn(0)
		if lexer.NewMatcher("EOF").Is(t) {
			break
		}

		if stop, eat := until(t); stop {
			if eat {
				p.advance()
			}
			break
		}

		p.advance()
		if !drop(t) {
			acc += t.Value
		}
	}

	if acc == "" {
		return nil, nil
	}
	return &Raw{Text: acc}, nil
}

func (p *Parser) varass(name Node, opt lexer.Token) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("varass", rerr)
		}
	}()

	Trim(name, func(s string) string {
		return strings.TrimSpace(s)
	})

	p.eat(lexer.NewMatcher("Char", " "))

	expr, err := p.raw(func(t lexer.Token) (bool, bool) {
		return NlMatcher.Is(t), true
	}, nil)
	if err != nil {
		return nil, err
	}

	if expr == nil {
		expr = &Raw{}
	}

	return &Var{
		Name:  name,
		Op:    opt.Value,
		Value: expr.Text,
	}, nil
}

func (p *Parser) ifeq(t lexer.Token) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("ifeq", rerr)
		}
	}()

	p.eatall(lexer.NewMatcher("Char", " "))

	_, err := p.expect(lexer.NewMatcher("Char", "("))
	if err != nil {
		return nil, err
	}

	left, err := p.expr(true, lexer.NewMatcher("Char", ","))
	if err != nil {
		return nil, err
	}
	if left == nil {
		left = &Raw{}
	}

	right, err := p.expr(true, lexer.NewMatcher("Char", ")"))
	if err != nil {
		return nil, err
	}
	if right == nil {
		right = &Raw{}
	}

	p.eatall(NlMatcher)

	body, err := p.ifbody()
	if err != nil {
		return nil, err
	}

	return &IfEq{
		Expected: t.Value == "ifeq",
		Left:     left,
		Right:    right,
		Body:     body,
	}, nil
}

func (p *Parser) ifdef(t lexer.Token) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("ifdef", rerr)
		}
	}()

	p.eatall(lexer.NewMatcher("Char", " "))

	ident, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	p.eatall(NlMatcher)

	body, err := p.ifbody()
	if err != nil {
		return nil, err
	}

	return &IfDef{
		Expected: t.Value == "ifdef",
		Ident:    ident,
		Body:     body,
	}, nil
}

func (p *Parser) ifbody() (_ []Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("ifbody", rerr)
		}
	}()

	body := make([]Node, 0)
	for {
		p.eatall(lexer.NewMultiMatcher(lexer.NewMatcher("Nl"), lexer.NewMatcher("Comment")))

		t := p.peekn(0)
		if isEOF(t) {
			return nil, p.err("unexpected eof")
		}

		if lexer.NewMatcher("Keyword", "endif").Is(t) {
			p.advance() // Eat endif
			return body, nil
		}

		n, err := p.root(t)
		if err != nil {
			return body, err
		}

		if n == nil {
			return body, p.ut(t)
		}

		body = append(body, n)
	}
}

func (p *Parser) recipe() ([]Node, error) {
	cmds := make([]Node, 0)
	for {
		if !p.eat(lexer.NewMatcher("Tab")) {
			break
		}

		if p.eat(lexer.NewMatcher("Comment")) {
			p.advance() // Eat \n
			continue
		}

		cmd, err := p.expr(true, NlMatcher)
		if err != nil {
			return cmds, err
		}
		if cmd == nil {
			return nil, p.ut(p.peekn(0))
		}

		cmds = append(cmds, cmd)
	}

	return cmds, nil
}

func (p *Parser) target(name Node) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("target", rerr)
		}
	}()

	p.eatall(lexer.NewMatcher("Char", " "))

	if name == nil {
		name = &Raw{}
	}

	expr, err := p.expr(false, lexer.NewMultiMatcher(
		NlMatcher,
		lexer.NewMatcher("Colon"),
	))
	if rerr != nil {
		return nil, err
	}

	t := p.advance() // Eat \n or :

	if lexer.NewMultiMatcher(NlMatcher, lexer.NewMatcher("EOF")).Is(t) {
		cmds, err := p.recipe()
		if rerr != nil {
			return nil, err
		}

		return &Target{
			Name:   name,
			Deps:   expr,
			Recipe: cmds,
		}, nil
	}

	prereq, err := p.expr(true, NlMatcher)
	if rerr != nil {
		return nil, err
	}

	cmds, err := p.recipe()
	if rerr != nil {
		return nil, err
	}

	return &StaticPatternTarget{
		Names:   name,
		Targets: expr,
		Prereqs: prereq,
		Recipe:  cmds,
	}, nil
}

func (p *Parser) expectIdent() (string, error) {
	ident, err := p.raw(func(t lexer.Token) (bool, bool) {
		if lexer.NewMatcher("Char").Is(t) {
			switch t.Value {
			case " ", "\n":
				return true, false
			}

			return false, false
		}

		return !lexer.NewMultiMatcher(
			lexer.NewMatcher("Keyword"),
			lexer.NewMatcher("Escaped"),
		).Is(t), false
	}, nil)
	if err != nil {
		return "", err
	}
	if ident == nil {
		return "", p.errat(p.peekn(0), "expected identifier")
	}
	return ident.Text, nil
}

func (p *Parser) define() (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("define", rerr)
		}
	}()

	p.eatall(lexer.NewMatcher("Char", " "))

	ident, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	p.eatall(lexer.NewMatcher("Char", " "))
	p.eat(NlMatcher)

	body, err := p.raw(func(t lexer.Token) (bool, bool) {
		return lexer.NewMatcher("Keyword", "endef").Is(t), true
	}, nil)
	if err != nil {
		return nil, err
	}

	Trim(body, func(s string) string {
		return strings.TrimSuffix(s, "\n")
	})

	return &Define{
		Name: ident,
		Body: body.Text,
	}, nil
}

func Trim(n Node, f func(s string) string) {
	if n == nil {
		return
	}

	last := n
	if expr, ok := last.(*Expr); ok {
		last = expr.Parts[len(expr.Parts)-1]
	}

	if raw, ok := last.(*Raw); ok {
		raw.Text = f(raw.Text)
	}
}
