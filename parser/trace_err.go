package parser

import (
	"errors"
	"strings"
)

type traceErr struct {
	trace []string
	err   error
}

func (e *traceErr) Error() string {
	prefix := strings.Join(e.trace, " > ")

	return "[" + prefix + "]: " + e.err.Error()
}

func (e *traceErr) Is(target error) bool {
	_, ok := target.(*traceErr)
	return ok
}

func (e *traceErr) Unwrap() error {
	return e.err
}

func Wrap(name string, err error) error {
	if errors.Is(err, &traceErr{}) {
		we := err.(*traceErr)
		we.trace = append([]string{name}, we.trace...)
		return we
	} else {
		return &traceErr{
			trace: []string{name},
			err:   err,
		}
	}
}
