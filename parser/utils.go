package parser

import (
	"fmt"
	"mxplrr/lexer"
)

func (p *Parser) advance() lexer.Token {
	if p.c > len(p.tokens)-1 {
		return lexer.NilToken
	}

	t := p.tokens[p.c]
	p.c++
	return t
}

func (p *Parser) peekn(i int) lexer.Token {
	if p.c+i > len(p.tokens)-1 {
		return lexer.NilToken
	}

	return p.tokens[p.c+i]
}

func (p *Parser) errat(t lexer.Token, f string, arg ...interface{}) error {
	args := append(arg, t.String())
	return p.err(f+" at %v", args...)
}

func (p *Parser) err(f string, args ...interface{}) error {
	return fmt.Errorf(f, args...)
}

// Unhandled token
func (p *Parser) ut(t lexer.Token) error {
	return p.errat(t, "unhandled token")
}

func (p *Parser) wrap(name string, err error) error {
	return Wrap(name, err)
}

func (p *Parser) expect(matcher lexer.Matcher) (lexer.Token, error) {
	t := p.advance()
	return t, matcher.Validate(t)
}

func (p *Parser) eat(matcher lexer.Matcher) bool {
	if matcher.Is(p.peekn(0)) {
		p.advance()
		return true
	}

	return false
}

func (p *Parser) eatall(matcher lexer.Matcher) bool {
	if p.eat(matcher) {
		for p.eat(matcher) {
			// Humm yummy...
		}
		return true
	}

	return false
}
