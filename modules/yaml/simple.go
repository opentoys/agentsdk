package yaml

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type node = map[string]any

func parseYAML(data string) (root map[string]any, e error) {
	root = node{}
	lines := strings.Split(strings.ReplaceAll(data, "\r", ""), "\n")

	type ctx struct {
		node  any
		depth int
	}

	stack := []ctx{{node: root, depth: -1}}

	for i, raw := range lines {
		trim := strings.TrimSpace(raw)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		depth := indent / 2

		if depth > len(stack)-1 {
			return nil, fmt.Errorf("line %d: bad indent", i+1)
		}
		stack = stack[:depth+1]

		// ---------- list item ----------
		if strings.HasPrefix(trim, "-") {
			top := stack[len(stack)-1].node
			list, ok := top.(*[]any)
			if !ok {
				return nil, fmt.Errorf("line %d: list under non-list", i+1)
			}

			val := strings.TrimSpace(trim[1:])
			if strings.Contains(val, ":") {
				m := node{}
				*list = append(*list, m)
				stack = append(stack, ctx{node: m, depth: depth + 1})
			} else {
				*list = append(*list, parseScalar(val))
			}
			continue
		}

		// ---------- mapping key ----------
		parts := strings.SplitN(trim, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: invalid syntax", i+1)
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		parent, ok := stack[len(stack)-1].node.(node)
		if !ok {
			return nil, fmt.Errorf("line %d: mapping under non-mapping", i+1)
		}

		// ---------- inline list ----------
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			items := parseInlineList(val[1 : len(val)-1])
			parent[key] = items
			continue
		}

		// ---------- block mapping / list ----------
		if val == "" {
			if i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "-") {
				lst := &[]any{}
				parent[key] = lst
				stack = append(stack, ctx{node: lst, depth: depth + 1})
			} else {
				m := node{}
				parent[key] = m
				stack = append(stack, ctx{node: m, depth: depth + 1})
			}
		} else {
			parent[key] = parseScalar(val)
		}
	}

	return root, nil
}

func parseInlineList(s string) []any {
	parts := strings.Split(s, ",")
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, parseScalar(p))
	}
	return out
}

func parseScalar(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

var Unmarshal = func(in []byte, out any) (e error) {
	data, e := parseYAML(string(in))
	if e != nil {
		return
	}
	buf, e := json.Marshal(data)
	if e != nil {
		return
	}
	e = json.Unmarshal(buf, out)
	return
}
