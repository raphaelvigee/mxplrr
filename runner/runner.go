package runner

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"makexplorer/parser"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Used for special variables
type Func struct {
	parser.Base
	Func func() (string, error)
}

func getEnv(data []string) map[string]parser.Node {
	items := make(map[string]parser.Node)
	for _, item := range data {
		splits := strings.Split(item, "=")
		key := splits[0]
		val := splits[1]

		items[key] = &parser.Raw{Text: val}
	}
	return items
}

func New() *Runner {
	env := getEnv(os.Environ())

	r := &Runner{
		Env:     env,
		Targets: map[string]*parser.Target{},
	}

	r.Env["MAKEFILE_LIST"] = &Func{Func: func() (string, error) {
		return strings.Join(r.files, " "), nil
	}}

	return r
}

type Runner struct {
	RootDir string
	Env     map[string]parser.Node
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

		log.Tracef("Left: %v Right: %v", left, right)

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

		log.Tracef("Defining target %v", name)

		if _, ok := r.Targets[name]; ok {
			return "", fmt.Errorf("target %v is already defined", name)
		}

		r.Targets[name] = n

		return "", nil
	case *parser.Var:
		name, err := r.Run(n.Name)
		if err != nil {
			return "", err
		}

		switch n.Op {
		case ":=", "::=":
			value, err := r.Run(n.Value)
			if err != nil {
				return "", err
			}

			log.Tracef("Defining simple var %v", name)

			r.Env[name] = &parser.Raw{Text: value}
			return "", nil
		case "+=":
			v := r.Env[name]

			var currentValue string
			if v != nil {
				currentValue, err = r.Run(v)
				if err != nil {
					return "", err
				}
				if currentValue != "" {
					currentValue += " "
				}
			}

			toAppend, err := r.Run(n.Value)
			if err != nil {
				return "", err
			}

			r.Env[name] = &parser.Raw{Text: currentValue + toAppend}
			return "", nil
		case "=":
			log.Tracef("Defining var %v", name)

			r.Env[name] = n.Value
			return "", nil
		default:
			return "", fmt.Errorf("unhandled op %s", n.Op)
		}
	case *parser.Define:
		log.Tracef("Define: %v", n.Name)

		r.Env[n.Name] = n.Body

		return "", nil
	}

	return "", fmt.Errorf("unhandled type %T", node)
}

func (r *Runner) RunWithVars(args map[string]parser.Node, f func() (string, error)) (string, error) {
	if len(args) == 0 {
		return f()
	}

	previous := r.Env
	newEnv := make(map[string]parser.Node)
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

func (r *Runner) RunVar(node parser.Node, args []string) (_ret string, _ error) {
	r.indent += "| "
	log.Tracef("%v> Running var %-14T - %v", r.indent, node, args)
	defer func() {
		log.Tracef("%v< %-14T -> %v", r.indent, node, _ret)
		r.indent = r.indent[2:]
	}()

	envArgs := make(map[string]parser.Node)
	for i, v := range args {
		envArgs[strconv.Itoa(i)] = &parser.Raw{Text: v}
	}

	return r.RunWithVars(envArgs, func() (string, error) {
		switch n := node.(type) {
		case *Func:
			return n.Func()
		}

		return r.Run(node)
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

		value, err := r.RunVar(name, nil)
		if err != nil {
			return "", err
		}

		return value, nil
	}

	args := exp.Parts[1:]
	switch root {
	case "shell":
		shcmd, err := r.RunNodes(args)
		if err != nil {
			return "", err
		}
		cmd := exec.Command("sh", "-c", strings.Join(shcmd, ","))
		data, err := cmd.CombinedOutput()
		out := string(data)
		if err != nil {
			return out, fmt.Errorf("shell error: %v with output:\n%v", err, string(data))
		}

		out = strings.TrimSuffix(out, "\n")

		return out, nil
	case "call":
		parts, err := r.RunNodes(args)
		if err != nil {
			return "", err
		}

		varName := parts[0]

		v := r.Env[varName]
		if v == nil {
			return "", fmt.Errorf("no var found for %v", varName)
		}

		return r.RunVar(v, parts)
	case "if":
		cond, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		if strings.TrimSpace(cond) != "" {
			return r.Run(args[1])
		}

		if len(args) == 3 {
			return r.Run(args[2])
		}

		return "", nil
	case "firstword":
		value, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		words := Words(value)
		if len(words) == 0 {
			return "", nil
		}

		return words[0], nil
	case "lastword":
		value, err := r.Run(args[0])
		if err != nil {
			return "", err
		}
		ss := Words(value)

		return ss[len(ss)-1], nil
	case "strip":
		value, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		return strings.TrimSuffix(strings.TrimPrefix(value, " "), " "), nil
	case "subst":
		values, err := r.RunNodes(args)
		if err != nil {
			return "", err
		}

		from := values[0]
		to := values[1]
		text := values[2]

		return strings.ReplaceAll(text, from, to), nil
	case "error", "warning", "info":
		text, err := r.RunNodesStr(args, ",")
		if err != nil {
			return "", err
		}

		fmt.Println(text)

		if root == "error" {
			return "", errors.New("error")
		}

		return "", nil
	case "eval":
		toEval, err := r.RunVar(args[0], nil)
		if err != nil {
			return "", err
		}

		n, err := parser.Parse(strings.NewReader(toEval))
		if err != nil {
			return "", err
		}

		_, err = r.Run(n)
		return "", err
	case "dir":
		s, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		paths := Words(s)
		for i, p := range paths {
			li := strings.LastIndex(p, "/")
			paths[i] = p[:li+1]
		}

		return strings.Join(paths, " "), nil
	case "notdir":
		s, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		paths := Words(s)
		for i, p := range paths {
			if !strings.Contains(p, "/") {
				continue
			}
			li := strings.LastIndex(p, "/")
			paths[i] = p[li+1:]
		}

		return strings.Join(paths, " "), nil
	case "realpath":
		s, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		paths := Words(s)
		for i, p := range paths {
			if !filepath.IsAbs(p) {
				p = filepath.Join(r.curdir(), p)
			}
			p, err = filepath.Abs(p)
			if err != nil {
				return "", err
			}
			paths[i] = filepath.Clean(p)
		}

		return strings.Join(paths, " "), nil
	case "filter", "filter-out":
		expected := root == "filter"

		patterns, err := r.Run(args[0])
		if err != nil {
			return "", err
		}
		text, err := r.Run(args[1])
		if err != nil {
			return "", err
		}

		out := make([]string, 0)
		for _, w := range Words(text) {
			match := false
			for _, pattern := range Words(patterns) {
				pattern := `^` + strings.Replace(regexp.QuoteMeta(pattern), "%", `(.*)`, 1) + `$`
				r, err := regexp.Compile(pattern)
				if err != nil {
					return "", err
				}

				if r.MatchString(w) {
					match = true
					break
				}
			}

			if match == expected {
				out = append(out, w)
			}
		}

		return strings.Join(out, " "), nil
	case "wildcard":
		pattern, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		if !filepath.IsAbs(pattern) {
			pattern = filepath.Join(r.curdir(), pattern)
		}

		files, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}

		return strings.Join(files, " "), nil
	case "foreach":
		targetVar, err := r.Run(args[0])
		if err != nil {
			return "", err
		}

		list, err := r.Run(args[1])
		if err != nil {
			return "", err
		}

		out := make([]string, 0)
		for _, w := range Words(list) {
			res, err := r.RunWithVars(map[string]parser.Node{
				targetVar: &parser.Raw{Text: w},
			}, func() (string, error) {
				return r.Run(args[2])
			})
			if err != nil {
				return "", err
			}
			out = append(out, res)
		}

		return strings.Join(out, " "), nil
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

	return strings.Join(outs, sep), nil
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
	if s == "" {
		return nil
	}

	return strings.Split(s, " ")
}
