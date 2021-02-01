package lexer

import (
	"fmt"
)

type Matcher interface {
	Is(t Token) bool
	Validate(t Token) error
}

func NewMatcher(name string, values ...string) Matcher {
	return crit{
		Name:   name,
		Values: values,
	}
}

type crit struct {
	Name   string
	Values []string
}

func (c crit) Is(t Token) bool {
	if t.Type != Symbol(c.Name) {
		return false
	}

	if len(c.Values) == 0 {
		return true
	}

	for _, v := range c.Values {
		if t.Value == v {
			return true
		}
	}

	return false
}

func (c crit) Validate(t Token) error {
	if !c.Is(t) {
		if len(c.Values) > 0 {
			return fmt.Errorf("expected `%v` with value in %v", c.Name, c.Values)
		}
		return fmt.Errorf("expected `%v`", c.Name)
	}
	return nil
}

type mcrit []Matcher

func (mc mcrit) Is(t Token) bool {
	for _, c := range mc {
		if c.Is(t) {
			return true
		}
	}
	return false
}

func (mc mcrit) Validate(t Token) error {
	if !mc.Is(t) {
		return fmt.Errorf("does not satisfy criterias")
	}
	return nil
}

func NewMultiMatcher(crits ...Matcher) Matcher {
	return mcrit(crits)
}
