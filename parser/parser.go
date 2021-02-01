package parser

import (
	"errors"
	"io"
	"makexplorer/lexer"
	"strings"
)

func isEOF(t lexer.Token) bool {
	return lexer.NewMatcher("EOF").Is(t)
}

func Parse(r io.Reader) (Node, error) {
	toks, err := lexer.Tokenize(r)
	if err != nil {
		return nil, err
	}

	return ParseTokens(toks)
}

func ParseTokens(tokens []lexer.Token) (Node, error) {
	return (&Parser{
		tokens: tokens,
	}).parse()
}

type Parser struct {
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
		p.advance()               // Eat modifier
		return p.root(p.peekn(0)) // TODO: Ignore modifier for now
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
	case lexer.Symbol("Keyword"), lexer.Symbol("Define"):
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

	exp, err := p.expr(false, lexer.NewMultiMatcher(lexer.NewMatcher("Nl"), lexer.NewMatcher("Colon"), lexer.NewMatcher("AssignOp")))
	if err != nil {
		return nil, err
	}
	if exp != nil {
		opt := p.peekn(0)
		switch opt.Type {
		case lexer.Symbol("Nl"):
			p.advance()
		case lexer.Symbol("Colon"):
			p.advance() // Eat :
			return p.target(exp)
		case lexer.Symbol("AssignOp"):
			p.advance() // Eat op
			return p.varass(exp, opt)
		}

		return exp, nil
	}

	return nil, p.ut(t)
}

func (p *Parser) parse() (Node, error) {
	file := &File{}

	for {
		t := p.peekn(0)
		n, err := p.root(t)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return file, err
		}
		file.Nodes = append(file.Nodes, n)
	}

	return file, nil
}

func (p *Parser) include() (*Include, error) {
	expr, err := p.expr(true, lexer.NewMatcher("Nl"))
	if err != nil {
		return nil, err
	}

	return &Include{
		Path: expr,
	}, nil
}

type UntilFunc func(token lexer.Token) (bool, bool)

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
			return o.rawMatcher.Is(t), false
		})
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

	if lexer.NewMatcher("ExpStart").Is(p.peekn(0)) {
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

			// Trailing comma
			if lexer.NewMatcher("Char", ",").Is(t) {
				if lexer.NewMatcher("ExpEnd").Is(p.peekn(1)) {
					p.advance() // Eat ,
					p.advance() // Eat )
					return exp, nil
				}
			}

			isFirst := len(exp.Parts) == 0
			sepMatcher := func() lexer.Matcher {
				if isFirst {
					return lexer.NewMatcher("Char", " ")
				}

				return lexer.NewMatcher("Char", ",")
			}()
			expMatcher := lexer.NewMultiMatcher(lexer.NewMatcher("ExpStart"), lexer.NewMatcher("ExpEnd"))

			part, err := p._expr(exprOptions{
				matcher:    sepMatcher,
				eat:        true,
				rawMatcher: lexer.NewMultiMatcher(expMatcher, sepMatcher),
			})
			if err != nil {
				return exp, err
			}

			if isFirst && part == nil {
				return exp, p.ut(t)
			}

			exp.Parts = append(exp.Parts, part)
		}
	}

	return nil, nil
}

func (p *Parser) raw(until UntilFunc) (_ *Raw, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("raw", rerr)
		}
	}()

	acc := ""
	for {
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
		acc += t.Value
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

	expr, err := p.expr(true, lexer.NewMatcher("Nl"))
	if err != nil {
		return nil, err
	}

	return &Var{
		Name:  name,
		Op:    opt.Value,
		Value: expr,
	}, nil
}

func (p *Parser) ifeq(t lexer.Token) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("ifeq", rerr)
		}
	}()

	_, err := p.expect(lexer.NewMatcher("Char", "("))
	if err != nil {
		return nil, err
	}

	left, err := p.expr(true, lexer.NewMatcher("Char", ","))
	if err != nil {
		return nil, err
	}

	right, err := p.expr(true, lexer.NewMatcher("Char", ")"))
	if err != nil {
		return nil, err
	}

	p.eatall(lexer.NewMatcher("Nl"))

	body, err := p.ifbody()
	if err != nil {
		return nil, err
	}

	return &IfEq{
		Not:   t.Value == "ifneq",
		Left:  left,
		Right: right,
		Body:  body,
	}, nil
}

func (p *Parser) ifdef(t lexer.Token) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("ifdef", rerr)
		}
	}()

	ident, err := p.expectIdent()
	if err != nil {
		return nil, err
	}

	p.eatall(lexer.NewMatcher("Nl"))

	body, err := p.ifbody()
	if err != nil {
		return nil, err
	}

	return &IfDef{
		Not:   t.Value == "ifndef",
		Ident: ident,
		Body:  body,
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

func (p *Parser) targetdeps() (_ []Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("targetdeps", rerr)
		}
	}()

	deps := make([]Node, 0)

	for {
		p.eatall(lexer.NewMatcher("Char", " "))
		t := p.peekn(0)
		if isEOF(t) {
			break
		}

		if lexer.NewMatcher("TargetDepsEnd").Is(t) {
			p.advance()
			break
		}

		expr, err := p.expr(false, lexer.NewMultiMatcher(
			lexer.NewMatcher("Char", " "),
			lexer.NewMatcher("TargetDepsEnd"),
		))
		if err != nil {
			return nil, err
		}
		if expr == nil {
			return nil, p.ut(t)
		}

		deps = append(deps, expr)
	}

	if len(deps) == 0 {
		return nil, nil
	}

	return deps, nil
}

func (p *Parser) target(name Node) (_ Node, rerr error) {
	defer func() {
		if rerr != nil {
			rerr = p.wrap("target", rerr)
		}
	}()

	target := &Target{
		Name: name,
	}

	deps, err := p.targetdeps()
	if err != nil {
		return target, err
	}
	target.Deps = deps

	for {
		t := p.peekn(0)
		if !lexer.NewMatcher("Tab").Is(t) {
			break
		}

		p.advance() // Eat \t
		cmd, err := p.expr(true, lexer.NewMatcher("Nl"))
		if err != nil {
			return target, err
		}

		target.Commands = append(target.Commands, cmd)
	}

	return target, nil
}

func (p *Parser) expectIdent() (string, error) {
	ident, err := p.raw(func(t lexer.Token) (bool, bool) {
		return !lexer.NewMultiMatcher(
			lexer.NewMatcher("Char"),
			lexer.NewMatcher("Keyword"),
			lexer.NewMatcher("Escaped"),
		).Is(t), false
	})
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

	body := make([]Node, 0)
	for {
		p.eatall(lexer.NewMatcher("Nl"))

		t := p.peekn(0)
		if isEOF(t) {
			return nil, p.err("unexpected eof")
		}

		if lexer.NewMatcher("Keyword", "endef").Is(t) {
			p.advance() // Eat endef
			break
		}

		p.eat(lexer.NewMatcher("Tab"))

		expr, err := p.expr(true, lexer.NewMatcher("Nl"))
		if err != nil {
			return nil, err
		}
		if expr == nil {
			return nil, p.ut(t)
		}

		body = append(body, expr)
	}

	return &Define{
		Name: ident,
		Body: body,
	}, nil
}
