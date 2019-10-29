package engine

import (
	"fmt"
	"runtime"

	yamlext "github.com/satoshi03/kubetpl/yaml"
	"gopkg.in/yaml.v2"
)

type ShellTemplate struct {
	content     []byte
	ignoreUnset bool
}

type ShellTemplateOption = func(*ShellTemplate) error

func ShellTemplateIgnoreUnset() ShellTemplateOption {
	return func(t *ShellTemplate) error {
		t.ignoreUnset = true
		return nil
	}
}

func NewShellTemplate(template []byte, options ...ShellTemplateOption) (Template, error) {
	tpl := ShellTemplate{template, false}
	for _, option := range options {
		if err := option(&tpl); err != nil {
			return nil, err
		}
	}
	return tpl, nil
}

func (t ShellTemplate) Render(data map[string]interface{}) (res []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	// ensure that input is a valid yaml even if expansion is done over the whole string
	// and not individual nodes (for now)
	for _, chunk := range yamlext.Chunk(t.content) {
		if err := yaml.Unmarshal(chunk, map[string]interface{}{}); err != nil {
			return nil, err
		}
	}
	r, err := envsubst(string(t.content), data, t.ignoreUnset)
	if err != nil {
		return nil, err
	}
	return []byte(r), nil
}

func envsubst(value string, env map[string]interface{}, ignoreUnset bool) (res string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(runtime.Error); ok {
				panic(r)
			}
			err = r.(error)
		}
	}()
	res = expandWithLineColumnInfo(value, func(key string, line int, col int) (string, bool) {
		if key == "$" || key == "" {
			return "$", true
		}
		value, ok := env[key]
		if !ok || value == nil {
			if ignoreUnset {
				return "", false
			}
			panic(fmt.Errorf("%d:%d: \"%s\" isn't set", line, col, key))
		}
		if !yamlext.IsBasicType(value) {
			panic(fmt.Errorf("%d:%d: \"%s\" must be either a string, number or a boolean", line, col, key))
		}
		return fmt.Sprintf("%v", value), true
	})
	return
}

func expandWithLineColumnInfo(s string, mapping func(string, int, int) (string, bool)) string {
	buf := make([]byte, 0, 2*len(s))
	i, l, n := 0, 0, 0
	for j := 0; j < len(s); j++ {
		if s[j] == '\n' {
			l++
			n = j + 1
		} else if s[j] == '$' && j+1 < len(s) {
			buf = append(buf, s[i:j]...)
			name, w := getShellName(s[j+1:])
			if v, ok := mapping(name, l+1, j-n+1); ok {
				buf = append(buf, v...)
			} else {
				buf = append(buf, s[j:j+1+w]...)
			}
			j += w
			i = j + 1
		}
	}
	return string(buf) + s[i:]
}

// Code below was taken from go/src/os/env.go

// Copyright 2010 The Go Authors.
// BSD 3-clause "New" or "Revised" License (https://github.com/golang/go/blob/master/LICENSE)

func isShellSpecialVar(c uint8) bool {
	switch c {
	case '*', '#', '$', '@', '!', '?', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	}
	return false
}

func isAlphaNum(c uint8) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func getShellName(s string) (string, int) {
	switch {
	case s[0] == '{':
		if len(s) > 2 && isShellSpecialVar(s[1]) && s[2] == '}' {
			return s[1:2], 3
		}
		for i := 1; i < len(s); i++ {
			if s[i] == '}' {
				return s[1:i], i + 1
			}
		}
		return "", 1
	case isShellSpecialVar(s[0]):
		return s[0:1], 1
	}
	var i int
	for i = 0; i < len(s) && isAlphaNum(s[i]); i++ {
	}
	return s[:i], i
}
