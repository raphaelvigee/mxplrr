package parser

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func parse(t *testing.T, s string) Node {
	node, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}

	return node
}

func TestParseTarget(t *testing.T) {
	n := parse(t, `
T.%/: $(A) test \
	bbbb
	@echo
`)
	assert.Equal(t, &Target{
		Name: &Raw{Text: "T.%/"},
		Deps: []Node{
			&Exp{
				Parts: []Node{
					&Raw{
						Text: "A",
					},
				},
			},
			&Raw{
				Text: "test",
			},
			&Raw{
				Text: "bbbb",
			},
		},
		Commands: []Node{
			&Raw{
				Text: "@echo",
			},
		},
	}, n)
}

func TestParseVar(t *testing.T) {
	n := parse(t, `
A=1
B = 2
`)
	assert.Equal(t, Nodes{
		&Var{
			Name: &Raw{
				Text: "A",
			},
			Op: "=",
			Value: &Raw{
				Text: "1",
			},
		},
		&Var{
			Name: &Raw{
				Text: "B",
			},
			Op: "=",
			Value: &Raw{
				Text: "2",
			},
		},
	}, n)
}

func TestParseIfdef(t *testing.T) {
	n := parse(t, `
ifndef AAA
AAA:=/test/some/path
endif
`)

	assert.Equal(t, &IfDef{
		Expected: false,
		Ident:    "AAA",
		Body: []Node{
			&Var{
				Name:  &Raw{Text: "AAA"},
				Op:    ":=",
				Value: &Raw{Text: "/test/some/path"},
			},
		},
	}, n)
}

func TestParseIfeq(t *testing.T) {
	n := parse(t, `
ifneq (AAA,BBB)
AAA=/test/some/path
endif
`)

	assert.Equal(t, &IfEq{
		Expected: false,
		Left:     &Raw{Text: "AAA"},
		Right:    &Raw{Text: "BBB"},
		Body: []Node{
			&Var{
				Name:  &Raw{Text: "AAA"},
				Op:    "=",
				Value: &Raw{Text: "/test/some/path"},
			},
		},
	}, n)
}

func TestParseInclude(t *testing.T) {
	n := parse(t, `
include $(VAR)/some-path.mk
`)
	assert.Equal(t, &Include{
		Path: &Expr{
			Parts: []Node{
				&Exp{
					Parts: []Node{
						&Raw{
							Text: "VAR",
						},
					},
				},
				&Raw{
					Text: "/some-path.mk",
				},
			},
		},
	}, n)
}

func TestParseNestedExp(t *testing.T) {
	n := parse(t, `
$(warning $(call ccyellow)SOME TEXT$(call ccend))
`)
	assert.Equal(t, &Exp{
		Parts: []Node{
			&Raw{
				Text: "warning",
			},
			&Expr{
				Parts: []Node{
					&Exp{
						Parts: []Node{
							&Raw{
								Text: "call",
							},
							&Raw{
								Text: "ccyellow",
							},
						},
					},
					&Raw{
						Text: "SOME TEXT",
					},
					&Exp{
						Parts: []Node{
							&Raw{
								Text: "call",
							},
							&Raw{
								Text: "ccend",
							},
						},
					},
				},
			},
		},
	}, n)
}

func TestParseComplexExp(t *testing.T) {
	n := parse(t, `
$(warning so me,more,$(ARG))
`)
	assert.Equal(t, &Exp{
		Parts: []Node{
			&Raw{
				Text: "warning",
			},
			&Raw{
				Text: "so me",
			},
			&Raw{
				Text: "more",
			},
			&Exp{
				Parts: []Node{
					&Raw{
						Text: "ARG",
					},
				},
			},
		},
	}, n)
}

func TestParseComments(t *testing.T) {
	n := parse(t, `
# One
# Long
# Comment
A=1

# A lonely comment

# Target comment
hello:
	world
`)
	assert.Equal(t, Nodes{
		&Var{
			Base: Base{
				"# One",
				"# Long",
				"# Comment",
			},
			Name: &Raw{
				Text: "A",
			},
			Op: "=",
			Value: &Raw{
				Text: "1",
			},
		},
		&Target{
			Base: Base{
				"# Target comment",
			},
			Name: &Raw{
				Text: "hello",
			},
			Commands: []Node{
				&Raw{
					Text: "world",
				},
			},
		},
	}, n)
}

func TestParseEOFTargetDeps(t *testing.T) {
	n := parse(t, `target: dep`)
	assert.Equal(t, &Target{
		Name: &Raw{Text: "target"},
		Deps: []Node{
			&Raw{
				Text: "dep",
			},
		},
	}, n)
}

func TestParseExprTarget(t *testing.T) {
	n := parse(t, `
$(ARG)-test:
	echo
`)
	assert.Equal(t, &Target{
		Name: &Expr{
			Parts: []Node{
				&Exp{
					Parts: []Node{
						&Raw{
							Text: "ARG",
						},
					},
				},
				&Raw{
					Text: "-test",
				},
			},
		},
		Commands: []Node{
			&Raw{
				Text: "echo",
			},
		},
	}, n)
}

func TestParseEmptyIfBody(t *testing.T) {
	n := parse(t, `
ifdef A
# AAA

# BBB

endif
`)
	assert.Equal(t, &IfDef{
		Expected: true,
		Ident:    "A",
		Body:     []Node{},
	}, n)
}

func TestParseTargetIshInDefine(t *testing.T) {
	n := parse(t, `
define  A  
"B: $(C)"
endef
`)
	assert.Equal(t, &Define{
		Name: "A",
		Body: &Expr{
			Parts: []Node{
				&Raw{
					Text: "\"B: ",
				},
				&Exp{
					Parts: []Node{
						&Raw{
							Text: "C",
						},
					},
				},
				&Raw{
					Text: "\"",
				},
			},
		},
	}, n)
}

func TestParseExpTrailingComma(t *testing.T) {
	n := parse(t, `
$(A $(B),)
`)
	assert.Equal(t, &Exp{
		Parts: []Node{
			&Raw{
				Text: "A",
			},
			&Exp{
				Parts: []Node{
					&Raw{
						Text: "B",
					},
				},
			},
			&Raw{
				Text: "",
			},
		},
	}, n)
}

func TestParseNlInExp(t *testing.T) {
	n := parse(t, `
$(A
$(B $(C),
D,)
)
`)
	assert.Equal(t, &Exp{
		Parts: []Node{
			&Raw{
				Text: "A",
			},
			&Expr{
				Parts: []Node{
					&Exp{
						Parts: []Node{
							&Raw{
								Text: "B",
							},
							&Exp{
								Parts: []Node{
									&Raw{
										Text: "C",
									},
								},
							},
							&Raw{
								Text: "\nD",
							},
							&Raw{
							},
						},
					},
					&Raw{
						Text: "\n",
					},
				},
			},
		},
	}, n)
}

func TestParseExpDigit(t *testing.T) {
	n := parse(t, `
$1
`)
	assert.Equal(t, &Exp{
		Parts: []Node{
			&Raw{
				Text: "1",
			},
		},
	}, n)
}
