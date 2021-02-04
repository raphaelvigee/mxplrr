package runner

import (
	"errors"
	"fmt"
	"mxplrr/parser"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type ExpFunc func(r *Runner, root string, args []parser.Node) (string, error)

var Exps map[string]ExpFunc

func init() {
	Exps = map[string]ExpFunc{
		"shell":      shell,
		"call":       call,
		"if":         _if,
		"words":      words,
		"firstword":  firstword,
		"lastword":   lastword,
		"strip":      strip,
		"subst":      subst,
		"eval":       eval,
		"dir":        dir,
		"notdir":     notdir,
		"basename":   basename,
		"realpath":   realpath,
		"wildcard":   wildcard,
		"foreach":    foreach,
		"filter":     filter,
		"filter-out": filter,
		"error":      control,
		"warning":    control,
		"info":       control,
		"patsubst":   patsubst,
		"addprefix":  addprefix,
		"value":      value,
	}
}

func value(r *Runner, root string, args []parser.Node) (string, error) {
	name, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	v := r.Env[name]
	if v == nil {
		return "", nil
	}

	return v.Value(r)
}

func addprefix(r *Runner, root string, args []parser.Node) (string, error) {
	prefix, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	namess, err := r.Run(args[1])
	if err != nil {
		return "", err
	}

	names := Words(namess)

	for i, n := range names {
		names[i] = prefix + n
	}

	return strings.Join(names, " "), nil
}

func basename(r *Runner, root string, args []parser.Node) (string, error) {
	text, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	words := Words(text)
	for i, w := range words {
		words[i] = filepath.Base(w)
	}

	return strings.Join(words, " "), nil
}

func words(r *Runner, root string, args []parser.Node) (string, error) {
	text, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	c := len(Words(text))

	return strconv.Itoa(c), nil
}

func shell(r *Runner, root string, args []parser.Node) (string, error) {
	shcmd, err := r.RunNodes(args)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("sh", "-c", strings.Join(shcmd, ","))
	cmd.Dir = r.RootDir
	data, err := cmd.CombinedOutput()
	out := string(data)
	if err != nil {
		return out, fmt.Errorf("shell error: %v with output:\n%v", err, string(data))
	}

	out = strings.TrimSuffix(out, "\n")

	return out, nil
}

func call(r *Runner, root string, args []parser.Node) (string, error) {
	parts, err := r.RunNodes(args)
	if err != nil {
		return "", err
	}

	varName := parts[0]

	v := r.Env[varName]
	if v == nil {
		return "", nil
	}

	return r.RunVar(v, parts)
}

func _if(r *Runner, root string, args []parser.Node) (string, error) {
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
}

func firstword(r *Runner, root string, args []parser.Node) (string, error) {
	value, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	words := Words(value)
	if len(words) == 0 {
		return "", nil
	}

	return words[0], nil
}

func lastword(r *Runner, root string, args []parser.Node) (string, error) {
	value, err := r.Run(args[0])
	if err != nil {
		return "", err
	}
	ss := Words(value)

	return ss[len(ss)-1], nil
}

func strip(r *Runner, root string, args []parser.Node) (string, error) {
	value, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	return strings.TrimSuffix(strings.TrimPrefix(value, " "), " "), nil
}

func subst(r *Runner, root string, args []parser.Node) (string, error) {
	values, err := r.RunNodes(args)
	if err != nil {
		return "", err
	}

	from := values[0]
	to := values[1]
	text := values[2]

	return strings.ReplaceAll(text, from, to), nil
}

func control(r *Runner, root string, args []parser.Node) (string, error) {
	text, err := r.RunNodesStr(args, ",")
	if err != nil {
		return "", err
	}

	fmt.Println(text)

	if root == "error" {
		return "", errors.New("error")
	}

	return "", nil
}

func eval(r *Runner, root string, args []parser.Node) (string, error) {
	value, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	p, err := parser.NewParserString(value)
	if err != nil {
		return "", err
	}

	n, err := p.Parse()
	if err != nil {
		return "", err
	}

	_, err = r.Run(n)

	return "", err
}

func dir(r *Runner, root string, args []parser.Node) (string, error) {
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
}

func notdir(r *Runner, root string, args []parser.Node) (string, error) {
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
}

func realpath(r *Runner, root string, args []parser.Node) (string, error) {
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
}

func filter(r *Runner, root string, args []parser.Node) (string, error) {
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
			r, err := toRegex(pattern)
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
}

func toRegex(pattern string) (*regexp.Regexp, error) {
	pattern = `^` + strings.Replace(regexp.QuoteMeta(pattern), "%", `(.*)`, 1) + `$`

	return regexp.Compile(pattern)
}

func wildcard(r *Runner, root string, args []parser.Node) (string, error) {
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
}

func foreach(r *Runner, root string, args []parser.Node) (string, error) {
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
		res, err := r.RunWithVars(map[string]Var{
			targetVar: ExpandVar(w),
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

func patsubst(r *Runner, root string, args []parser.Node) (string, error) {
	pattern, err := r.Run(args[0])
	if err != nil {
		return "", err
	}

	repl, err := r.Run(args[1])
	if err != nil {
		return "", err
	}

	text, err := r.Run(args[2])
	if err != nil {
		return "", err
	}

	if !strings.Contains(pattern, "%") {
		pattern = "%" + pattern
		repl = "%" + repl
	}

	reg, err := toRegex(pattern)
	if err != nil {
		return "", err
	}

	words := Words(text)
	for i, w := range words {
		if reg.MatchString(w) {
			words[i] = reg.ReplaceAllStringFunc(w, func(s string) string {

				if !strings.Contains(repl, "%") {
					return repl
				}

				groups := reg.FindStringSubmatch(w)
				return strings.Replace(repl, "%", groups[1], 1)
			})
		}
	}

	return strings.Join(words, " "), nil
}
