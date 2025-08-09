package middleware

// This is a near verbatim transplant of chi/middleware.Recoverer, but
// adapted to Huma’s (ctx, next) signature and `huma.WriteErr` response.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// Recover catches panics, logs a pretty stack trace, and returns the
// usual Huma error envelope `{error:"Internal server error",status:500}`.
func (self *Middleware) Recoverer(ctx huma.Context, next func(huma.Context)) {
	defer func() {
		if rvr := recover(); rvr != nil {
			// Honour http.ErrAbortHandler (same as chi).
			if rvr == http.ErrAbortHandler {
				panic(rvr)
			}

			// Pretty print stack
			PrintPrettyStack(rvr)

			// Skip WebSocket/upgrade connections, just like chi.
			if ctx.Header("Connection") != "Upgrade" {
				huma.WriteErr(self.api, ctx, http.StatusInternalServerError, "Internal server error")
			}
		}
	}()

	next(ctx)
}

// * Lifted from go-chi middleware.Recoverer

var recovererErrorWriter io.Writer = os.Stderr

func PrintPrettyStack(rvr interface{}) {
	debugStack := debug.Stack()
	s := prettyStack{}
	out, err := s.parse(debugStack, rvr)
	if err == nil {
		recovererErrorWriter.Write(out)
	} else {
		// Fallback to the raw stack.
		os.Stderr.Write(debugStack)
	}
}

type prettyStack struct{}

func (s prettyStack) parse(debugStack []byte, rvr interface{}) ([]byte, error) {
	var err error
	useColor := true
	buf := &bytes.Buffer{}

	cW(buf, false, bRed, "\n")
	cW(buf, useColor, bCyan, " panic: ")
	cW(buf, useColor, bBlue, "%v", rvr)
	cW(buf, false, bWhite, "\n \n")

	stack := strings.Split(string(debugStack), "\n")
	lines := []string{}

	// Find the innermost panic.
	for i := len(stack) - 1; i > 0; i-- {
		lines = append(lines, stack[i])
		if strings.HasPrefix(stack[i], "panic(") {
			lines = lines[:len(lines)-2] // chop boilerplate
			break
		}
	}

	// Reverse the order.
	for i := len(lines)/2 - 1; i >= 0; i-- {
		opp := len(lines) - 1 - i
		lines[i], lines[opp] = lines[opp], lines[i]
	}

	// Decorate lines.
	for i, line := range lines {
		lines[i], err = s.decorateLine(line, useColor, i)
		if err != nil {
			return nil, err
		}
	}

	for _, l := range lines {
		fmt.Fprint(buf, l)
	}
	return buf.Bytes(), nil
}

func (s prettyStack) decorateLine(line string, useColor bool, num int) (string, error) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "\t"), strings.Contains(line, ".go:"):
		return s.decorateSourceLine(line, useColor, num)
	case strings.HasSuffix(line, ")"):
		return s.decorateFuncCallLine(line, useColor, num)
	case strings.HasPrefix(line, "\t"):
		return strings.Replace(line, "\t", "      ", 1), nil
	default:
		return fmt.Sprintf("    %s\n", line), nil
	}
}

func (s prettyStack) decorateFuncCallLine(line string, useColor bool, num int) (string, error) {
	idx := strings.LastIndex(line, "(")
	if idx < 0 {
		return "", errors.New("not a func call line")
	}

	buf := &bytes.Buffer{}
	pkg := line[:idx]
	method := ""

	if pos := strings.LastIndex(pkg, string(os.PathSeparator)); pos < 0 {
		if pos := strings.Index(pkg, "."); pos > 0 {
			method = pkg[pos:]
			pkg = pkg[:pos]
		}
	} else {
		method = pkg[pos+1:]
		pkg = pkg[:pos+1]
		if pos := strings.Index(method, "."); pos > 0 {
			pkg += method[:pos]
			method = method[pos:]
		}
	}

	pkgColor := nYellow
	methodColor := bGreen

	if num == 0 {
		cW(buf, useColor, bRed, " -> ")
		pkgColor, methodColor = bMagenta, bRed
	} else {
		cW(buf, useColor, bWhite, "    ")
	}
	cW(buf, useColor, pkgColor, "%s", pkg)
	cW(buf, useColor, methodColor, "%s\n", method)
	return buf.String(), nil
}

func (s prettyStack) decorateSourceLine(line string, useColor bool, num int) (string, error) {
	idx := strings.LastIndex(line, ".go:")
	if idx < 0 {
		return "", errors.New("not a source line")
	}

	buf := &bytes.Buffer{}
	path, lineno := line[:idx+3], line[idx+3:]

	pos := strings.LastIndex(path, string(os.PathSeparator))
	dir, file := path[:pos+1], path[pos+1:]

	if pos := strings.Index(lineno, " "); pos > 0 {
		lineno = lineno[:pos]
	}

	fileColor, lineColor := bCyan, bGreen
	if num == 1 {
		cW(buf, useColor, bRed, " ->   ")
		fileColor, lineColor = bRed, bMagenta
	} else {
		cW(buf, false, bWhite, "      ")
	}
	cW(buf, useColor, bWhite, "%s", dir)
	cW(buf, useColor, fileColor, "%s", file)
	cW(buf, useColor, lineColor, "%s", lineno)
	if num == 1 {
		cW(buf, false, bWhite, "\n")
	}
	cW(buf, false, bWhite, "\n")
	return buf.String(), nil
}
