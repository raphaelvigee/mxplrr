package runner

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"mxplrr/parser"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func makeRun(t *testing.T, pre, s string) string {
	f, err := ioutil.TempFile("", "make-run")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(pre + `

run:
	@echo ` + s + `
`)

	cmd := exec.Command("make", "-f", f.Name(), "run")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, out)
	}

	return strings.TrimSuffix(string(out), "\n")
}

func run(t *testing.T, r *Runner, ss ...string) string {
	var out string
	for _, s := range ss {
		p, err := parser.NewParserString(s)
		if err != nil {
			t.Fatal(err)
		}

		n, err := p.Parse()
		if err != nil {
			t.Fatal(err)
		}

		out, err = r.Run(n)
		if err != nil {
			t.Fatal(err)
		}
	}

	return out
}

func TestRunner_RunExpGetVariable(t *testing.T) {
	r := &Runner{
		Env: map[string]Var{
			"TEST": RawVar("value"),
		},
	}
	out := run(t, r, `$(TEST)`)
	assert.Equal(t, "value", out)
}

func TestRunner_RunExpSetVariable(t *testing.T) {
	r := &Runner{
		Env: map[string]Var{
			"TEST": RawVar("value"),
		},
	}
	run(t, r, `TEST2:=$(TEST)`)
	assert.Equal(t, RawVar("value"), r.Env["TEST2"])
}

func TestRunner_RunShell(t *testing.T) {
	r := &Runner{}
	out := run(t, r, `$(shell echo "hello\nworld")`)
	assert.Equal(t, "hello\nworld", out)
}

func TestRunner_ComplexDefine(t *testing.T) {
	r := &Runner{
		Env: map[string]Var{},
	}
	run(t, r, `
MODULES =

define somedefine
$(eval
MODULES += $1
$(1)-path:=somepath
)
endef

$(call somedefine,somemodule)

`)
	assert.Equal(t, RawVar("somepath"), r.Env["somemodule-path"])
}

func TestRunner_RunExpr(t *testing.T) {
	r := &Runner{
		Env: map[string]Var{
			"TEST":       RawVar("value"),
			"test-value": RawVar("42"),
		},
	}
	out := run(t, r, `$(test-$(TEST))`)
	assert.Equal(t, "42", out)
}

const rootDir = "/some/dir"

func runAsFile(t *testing.T, s ...string) string {
	r := &Runner{
		RootDir: rootDir,
		Env:     map[string]Var{},
		files:   []string{rootDir + "/subdir/Makefile"},
	}

	return run(t, r, s...)
}

func TestRunner_Firstword(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(firstword a b c)"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Lastword(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(lastword a b c)"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Dir(t *testing.T) {
	testCases := []struct {
		path string
	}{
		{"/file.txt"},
		{"test/file.txt"},
		{"/some/test/file.txt"},
		{"/some/test/"},
		{"/some/test"},
		{"/"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, "$(dir "+tc.path+")")
		expected := makeRun(t, "", "$(dir "+tc.path+")")
		assert.Equal(t, expected, out, "dir "+tc.path)

		out = runAsFile(t, "$(notdir "+tc.path+")")
		expected = makeRun(t, "", "$(notdir "+tc.path+")")
		assert.Equal(t, expected, out, "notdir "+tc.path)
	}
}

func TestRunner_Realpath(t *testing.T) {
	testCases := []struct {
		expr     string
		expected string
	}{
		{"$(realpath file.txt)", "/some/dir/subdir/file.txt"},
		{"$(realpath ./file.txt)", "/some/dir/subdir/file.txt"},
		{"$(realpath ../file.txt)", "/some/dir/file.txt"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.Equal(t, tc.expected, out, tc.expr)
	}
}

func TestRunner_Filter(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(filter %.c %.s,foo.c bar.c baz.s ugh.h)"},
		{"$(filter-out %.c %.s,foo.c bar.c baz.s ugh.h)"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Strip(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(strip a b c )"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Wildcard(t *testing.T) {
	d, err := ioutil.TempDir("", "mxplrr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	for _, f := range []string{"some", "file", "mxplrr_test_wildcard.ext"} {
		_, err := os.Create(d + "/" + f)
		if err != nil {
			t.Fatal(err)
		}
	}

	testCases := []struct {
		expr string
	}{
		{"$(wildcard " + d + "/*)"},
		{"$(wildcard " + d + "/*.ext)"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Foreach(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(foreach dir,a b c d,$(dir))"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Subst(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(subst ee,EE,feet on the street)"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Patsubst(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(patsubst %.c,%.o,$(foo))"},
		{"$(foo:.o=.c)"},
		{"$(foo:.o=%.c)"},
		{"$(foo:%.o=%.c)"},
	}
	pre := `foo := a.o b.o l.a c.o`
	for _, tc := range testCases {
		out := runAsFile(t, pre, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, pre, tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Addprefix(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(addprefix src/,foo bar)"},
	}
	for _, tc := range testCases {
		out := runAsFile(t, "", tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, "", tc.expr)
		assert.Equal(t, expected, out)
	}
}

func TestRunner_Call(t *testing.T) {
	testCases := []struct {
		expr string
	}{
		{"$(bar)"},
	}
	pre := `
comma:= ,
empty:=
space:= $(empty) $(empty)
foo:= a b c
bar:= $(subst $(space),$(comma),$(foo))`[1:]
	for _, tc := range testCases {
		out := runAsFile(t, pre, tc.expr)
		assert.NotEmpty(t, out)
		expected := makeRun(t, pre, tc.expr)
		assert.Equal(t, expected, out)
	}
}
