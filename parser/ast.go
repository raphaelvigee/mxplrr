package parser

type Node interface {
	SetComments(comments []string)
	Comments() []string
}

type Base []string

func (b *Base) SetComments(comments []string) {
	*b = comments
}

func (b Base) Comments() []string {
	return b
}

type File struct {
	Base
	Path  string
	Nodes Nodes
}

type Target struct {
	Base
	Name   Node
	Deps   Node
	Recipe []Node
}

type StaticPatternTarget struct {
	Base
	Names   Node
	Targets Node
	Prereqs Node
	Recipe  []Node
}

type Raw struct {
	Base
	Text string
}

type Expr struct {
	Base
	Parts []Node
}

type Exp struct {
	Base
	Parts []Node
}

type Var struct {
	Base
	Name  Node
	Op    string
	Value Node
}

type PatSubst struct {
	Base
	Name    Node
	Pattern Node
	Subst   Node
}

type Include struct {
	Base
	Path Node
}

type IfEq struct {
	Base
	Expected bool
	Left     Node
	Right    Node
	Body     []Node
}

type IfDef struct {
	Base
	Expected bool
	Ident    string
	Body     []Node
}

type Comment struct {
	Base
	Text string
}

type Define struct {
	Base
	Name string
	Body Node
}

type Modifier struct {
	Base
	Modifier string
	Node     Node
}

type Nodes []Node

func (b Nodes) SetComments(comments []string) {
	b[0].SetComments(comments)
}

func (b Nodes) Comments() []string {
	return nil
}
