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
	Nodes []Node
}

type Target struct {
	Base
	Name     Node
	Deps     []Node
	Commands []Node
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

type Include struct {
	Base
	Path Node
}

type IfEq struct {
	Base
	Not   bool
	Left  Node
	Right Node
	Body  []Node
}

type IfDef struct {
	Base
	Not   bool
	Ident string
	Body  []Node
}

type Comment struct {
	Base
	Text string
}

type Define struct {
	Base
	Name string
	Body []Node
}
