package runner

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"mxplrr/parser"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Var interface {
	Get(r *Runner) (string, error)
	Value(r *Runner) (string, error)
}

type RawVar string

func (v RawVar) Get(*Runner) (string, error) {
	return string(v), nil
}

func (v RawVar) Value(r *Runner) (string, error) {
	return v.Get(r)
}

type ExpandVar string

func (v ExpandVar) Get(r *Runner) (string, error) {
	return RunExprFromString(r, string(v))
}

func (v ExpandVar) Value(*Runner) (string, error) {
	return string(v), nil
}

type FuncVar func(r *Runner) (string, error)

func (f FuncVar) Get(r *Runner) (string, error) {
	return f(r)
}

func (f FuncVar) Value(r *Runner) (string, error) {
	return f.Get(r)
}

func RunExprFromString(r *Runner, s string) (string, error) {
	p, err := parser.NewParserString(s)
	if err != nil {
		return "", err
	}

	n, err := p.ParseExpr()
	if err != nil {
		return "", err
	}

	if n == nil {
		return "", nil
	}

	return r.Run(n)
}

// Used for special variables
type Func struct {
	parser.Base
	Func func() (string, error)
}

func getEnv(data []string) map[string]Var {
	items := make(map[string]Var)
	for _, item := range data {
		splits := strings.Split(item, "=")
		key := splits[0]
		val := splits[1]

		items[key] = RawVar(val)
	}
	return items
}

func New() *Runner {
	env := getEnv(os.Environ())
	env["MAKEFILE_LIST"] = FuncVar(func(r *Runner) (string, error) {
		return strings.Join(r.files, " "), nil
	})

	return &Runner{
		Env:     env,
		Targets: map[string]*parser.Target{},
	}
}

type Runner struct {
	RootDir string
	Env     map[string]Var
	Targets map[string]*parser.Target

	files                []string
	indent               string
	reportedFailurePoint bool
}

func (r *Runner) Include(file *parser.File) error {
	log.Tracef("%v> Include %v", r.indent, file.Path)

	r.files = append(r.files, file.Path)
	_, err := r.Run(file.Nodes)

	return err
}

func (r *Runner) Run(node parser.Node) (_ret string, _err error) {
	r.indent += "| "
	log.Tracef("%v> %-15T", r.indent, node)
	defer func() {
		if _err != nil && !r.reportedFailurePoint {
			r.reportedFailurePoint = true
			log.Errorf("%v| ERROR happened here", r.indent)
		}
		log.Tracef("%v< %-15T -> `%v`", r.indent, node, _ret)
		r.indent = r.indent[2:]
	}()

	switch n := node.(type) {
	case parser.Nodes:
		return r.RunNodesStr(n, "\n")
	case *parser.Modifier:
		switch n.Modifier {
		case "-":
			s, _ := r.Run(n.Node)
			return s, nil
		default:
			return "", fmt.Errorf("unhandled modifier %v", n.Modifier)
		}
	case *parser.Raw:
		return n.Text, nil
	case *parser.Expr:
		return r.RunNodesStr(n.Parts, "")
	case *parser.Exp:
		return r.runExp(n)
	case *parser.IfEq:
		left, err := r.Run(n.Left)
		if err != nil {
			return "", err
		}

		right, err := r.Run(n.Right)
		if err != nil {
			return "", err
		}

		log.Tracef("Left: `%v` Right: `%v`", left, right)

		if (left == right) == n.Expected {
			_, err = r.RunNodes(n.Body)
		}

		return "", err
	case *parser.IfDef:
		log.Tracef("Ident: %v", n.Ident)

		var err error
		if _, ok := r.Env[n.Ident]; ok == n.Expected {
			_, err = r.RunNodes(n.Body)
		} else {
			log.Tracef("Not found")
		}

		return "", err
	case *parser.Include:
		filenames, err := r.Run(n.Path)
		if err != nil {
			return "", err
		}

		allFilenames := make([]string, 0)
		for _, f := range Words(filenames) {
			if strings.Contains(f, "*") {
				files, _ := filepath.Glob(f)

				allFilenames = append(allFilenames, files...)
			} else {
				allFilenames = append(allFilenames, f)
			}
		}

		for _, filename := range allFilenames {
			included, err := r.include(filename)
			if err != nil {
				return "", err
			}

			err = r.Include(included)
			if err != nil {
				return "", err
			}
		}

		return "", nil
	case *parser.Target:
		name, err := r.Run(n.Name)
		if err != nil {
			return "", err
		}

		if name == "" {
			return "", nil
		}

		log.Tracef("Defining target %v", name)

		r.Targets[name] = n

		return "", nil
	case *parser.Var:
		name, err := r.Run(n.Name)
		if err != nil {
			return "", err
		}

		switch n.Op {
		case ":=", "::=":
			log.Tracef("Defining simple var %v", name)

			v, err := RunExprFromString(r, n.Value)
			if err != nil {
				return "", err
			}

			r.Env[name] = RawVar(v)

			return "", nil
		case "+=":
			v := r.Env[name]

			var currentValue string
			if v != nil {
				currentValue, err = v.Get(r)
				if err != nil {
					return "", err
				}
				if currentValue != "" {
					currentValue += " "
				}
			}

			toAppend, err := RunExprFromString(r, n.Value)
			if err != nil {
				return "", err
			}

			r.Env[name] = RawVar(currentValue + toAppend)
			return "", nil
		case "?=":
			if _, ok := r.Env[name]; ok {
				log.Tracef("Defining var %v", name)

				value, err := RunExprFromString(r, n.Value)
				if err != nil {
					return "", err
				}

				r.Env[name] = RawVar(value)
			}

			return "", nil
		case "=":
			log.Tracef("Defining var %v", name)

			r.Env[name] = ExpandVar(n.Value)

			return "", nil
		default:
			return "", fmt.Errorf("unhandled op %s", n.Op)
		}
	case *parser.Define:
		log.Tracef("Define: %v", n.Name)

		r.Env[n.Name] = ExpandVar(n.Body)

		return "", nil
	case *parser.PatSubst:
		return Exps["patsubst"](r, "patsubst", []parser.Node{
			n.Pattern,
			n.Subst,
			&parser.Exp{
				Parts: []parser.Node{n.Name},
			},
		})
	case *parser.StaticPatternTarget:
		// TODO: implement StaticPatternTarget
		return "", nil
	}

	return "", fmt.Errorf("unhandled type %T", node)
}

func (r *Runner) RunWithVars(args map[string]Var, f func() (string, error)) (string, error) {
	if len(args) == 0 {
		return f()
	}

	previous := r.Env
	newEnv := make(map[string]Var)
	for k, v := range r.Env {
		newEnv[k] = v
	}
	for k, v := range args {
		newEnv[k] = v
	}
	r.Env = newEnv

	s, err := f()

	// Restore previous env
	for k := range args {
		delete(r.Env, k)
		if pv, ok := previous[k]; ok {
			r.Env[k] = pv
		}
	}

	return s, err
}

func (r *Runner) RunVar(v Var, args []string) (_ret string, _ error) {
	r.indent += "| "
	log.Tracef("%v> Running var %-14T - %v", r.indent, v, args)
	defer func() {
		log.Tracef("%v< %-14T -> %v", r.indent, v, _ret)
		r.indent = r.indent[2:]
	}()

	envArgs := make(map[string]Var)
	for i, v := range args {
		envArgs[strconv.Itoa(i)] = RawVar(v)
	}

	return r.RunWithVars(envArgs, func() (string, error) {
		return v.Get(r)
	})
}

func (r *Runner) runExp(exp *parser.Exp) (string, error) {
	root, err := r.Run(exp.Parts[0])
	if err != nil {
		return "", err
	}

	r.indent += "| "
	log.Tracef("%v> EXP: %v", r.indent, root)
	defer func() {
		log.Tracef("%v<%v", r.indent, root)
		r.indent = r.indent[2:]
	}()

	if len(exp.Parts) == 1 {
		log.Tracef("Var expansion %v", root)

		name := r.Env[root]
		if name == nil {
			log.Warnf("Undefined var %v", root)
			return "", nil
		}

		value, err := name.Get(r)
		if err != nil {
			return "", err
		}

		return value, nil
	}

	if f, ok := Exps[root]; ok {
		args := exp.Parts[1:]

		return f(r, root, args)
	}

	return "", fmt.Errorf("unhandled exp `%v`", root)
}

func (r *Runner) RunNodes(nodes []parser.Node) ([]string, error) {
	out := make([]string, 0)

	for _, n := range nodes {
		s, err := r.Run(n)
		if err != nil {
			return out, err
		}

		out = append(out, s)
	}

	return out, nil
}

func (r *Runner) RunNodesStr(nodes []parser.Node, sep string) (string, error) {
	outs, err := r.RunNodes(nodes)
	if err != nil {
		return "", err
	}

	neouts := make([]string, 0)
	for _, out := range outs {
		if out != "" {
			neouts = append(neouts, out)
		}
	}

	return strings.Join(neouts, sep), nil
}

func (r *Runner) curdir() string {
	return filepath.Dir(r.files[len(r.files)-1])
}

func (r *Runner) include(filename string) (*parser.File, error) {
	if !filepath.IsAbs(filename) {
		siblingFilename := filepath.Join(r.curdir(), filename)
		log.Tracef("Trying to include %v", siblingFilename)

		included, err := parser.ParseFile(siblingFilename)
		if err == nil {
			return included, nil
		}

		if !os.IsNotExist(err) {
			return nil, err
		}

		filename = filepath.Join(r.RootDir, filename)

		log.Tracef("Fallback to %v", filename)
	}

	return parser.ParseFile(filename)
}

func Words(s string) []string {
	s = strings.TrimSpace(s)

	if s == "" {
		return nil
	}

	return strings.Split(s, " ")
}
