package migrate

import (
	"regexp"

	"gopkg.in/yaml.v3"
	"strconv"
	"strings"
)

var reBlockScalarHeader = regexp.MustCompile(`^(\s*)([^\s#][^:]*):\s*\|[-+]?\s*$`)
var reInlineValue = regexp.MustCompile(`^(\s*)([^\s#][^:]*):[ \t]+(\S.*)$`)

type textItem struct {
	lines     []string
	directive []string
}

func (c *templateConverter) rewriteTextBlocks(src, filename string) string {
	scope := c.fileScope(src, filename)
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))

	for i := 0; i < len(lines); i++ {
		m := reBlockScalarHeader.FindStringSubmatch(lines[i])
		if nil == m {
			if inline := c.inlineValueText(lines[i], filename, scope); nil != inline {
				out = append(out, inline...)
				continue
			}
			out = append(out, lines[i])
			continue
		}
		if inline := c.inlineValueText(lines[i], filename, scope); nil != inline {
			out = append(out, inline...)
			continue
		}
		body, next := blockScalarBody(lines, i+1, len(m[1]))
		if 0 == len(body) || !strings.Contains(strings.Join(body, "\n"), "{{") {
			out = append(out, lines[i])
			continue
		}
		rendered, ok := c.textBlockYAML(body, m[1], filename, scope)
		if !ok {
			out = append(out, lines[i])
			continue
		}
		out = append(out, m[1]+strings.TrimSpace(m[2])+":")
		out = append(out, rendered...)
		for j := i + 1; j < next; j++ {
			out = append(out, "")
		}
		i = next - 1
	}
	return strings.Join(out, "\n")
}

func (c *templateConverter) fileScope(src, filename string) *scope {
	sc := c.rootScope(filename)
	nodes, err := buildNodes(src)
	if nil != err {
		return sc
	}
	probe := newEmitter(filename, &MigrateResult{})
	probe.textOnly = true
	probe.emit(nodes, sc)
	return sc
}

func (c *templateConverter) inlineValueText(line, filename string, sc *scope) []string {
	m := reInlineValue.FindStringSubmatch(line)
	if nil == m || !strings.Contains(m[3], "{{") {
		return nil
	}
	nodes, err := buildNodes(m[3])
	if nil != err || !hasInlineBlock(nodes) {
		return nil
	}
	probe := newEmitter(filename, &MigrateResult{})
	probe.textOnly = true
	if _, exprErr := probe.inlineNodesExpr(nodes, sc.child("")); nil == exprErr {
		return nil
	}
	e := newEmitter(filename, c.result)
	e.textOnly = true
	items, err := e.textItems(nodes, sc.child(""))
	if nil != err || 0 == len(items) {
		return nil
	}
	indent := m[1]
	if 1 == len(items) && 0 < len(items[0].directive) && strings.HasPrefix(items[0].directive[0], "$text:") {
		out := []string{indent + strings.TrimSpace(m[2]) + ":"}
		for _, entry := range items[0].directive {
			out = append(out, indent+"  "+entry)
		}
		return out
	}
	out := []string{indent + strings.TrimSpace(m[2]) + ":", indent + "  $text:"}
	for _, item := range items {
		if 0 < len(item.directive) {
			for j, entry := range item.directive {
				lead := indent + "      "
				if 0 == j {
					lead = indent + "    - "
				}
				out = append(out, lead+entry)
			}
			continue
		}
		for _, entry := range item.lines {
			out = append(out, indent+"    - "+quoteYAML(entry))
		}
	}
	return append(out, indent+"  $join: \"\"")
}

func hasInlineBlock(nodes []node) bool {
	for _, n := range nodes {
		if _, ok := n.(*blockNode); ok {
			return true
		}
	}
	return false
}

func blockScalarBody(lines []string, start, headerIndent int) ([]string, int) {
	body := make([]string, 0, 8)
	i := start
	for ; i < len(lines); i++ {
		line := lines[i]
		if "" == strings.TrimSpace(line) {
			body = append(body, "")
			continue
		}
		if len(leadingWhitespace(line)) <= headerIndent {
			break
		}
		body = append(body, line)
	}
	for len(body) > 0 && "" == strings.TrimSpace(body[len(body)-1]) {
		body = body[:len(body)-1]
		i--
	}
	return body, i
}

func (c *templateConverter) textNodeFor(nodes []node, filename string) (*yaml.Node, bool) {
	e := newEmitter(filename, c.result)
	e.textOnly = true
	sc := c.rootScope(filename)
	nodes = e.collectAccumulators(nodes, sc)
	items, err := e.textItems(nodes, sc)
	if nil != err {
		c.lastTextError = err.Error()
		return nil, false
	}
	if 0 == len(items) {
		c.lastTextError = "text body produced no lines"
		return nil, false
	}
	indent := ""
	lines := letLines(e.lets, "")
	if 0 < len(lines) {
		indent = "  "
	}
	lines = append(lines, indent+"$text:")
	for _, item := range items {
		if 0 < len(item.directive) {
			for j, line := range item.directive {
				prefix := "    "
				if 0 == j {
					prefix = "  - "
				}
				lines = append(lines, prefix+line)
			}
			continue
		}
		for _, line := range item.lines {
			lines = append(lines, "  - "+quoteYAML(line))
		}
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &parsed); nil != err || 0 == len(parsed.Content) {
		return nil, false
	}
	return parsed.Content[0], true
}

func (c *templateConverter) textBlockYAML(body []string, indent, filename string, sc *scope) ([]string, bool) {
	text := dedent(body)
	nodes, err := buildNodes(text)
	if nil != err {
		return nil, false
	}
	e := newEmitter(filename, c.result)
	e.textOnly = true
	items, err := e.textItems(nodes, sc.child(""))
	if nil != err || 0 == len(items) {
		return nil, false
	}

	out := make([]string, 0, len(items)+2)
	out = append(out, indent+"  $text:")
	for _, item := range items {
		if 0 < len(item.directive) {
			for j, line := range item.directive {
				prefix := indent + "      "
				if 0 == j {
					prefix = indent + "    - "
				}
				out = append(out, prefix+line)
			}
			continue
		}
		for _, line := range item.lines {
			out = append(out, indent+"    - "+quoteYAML(line))
		}
	}
	return out, true
}

func dedent(lines []string) string {
	width := -1
	for _, line := range lines {
		if "" == strings.TrimSpace(line) {
			continue
		}
		lead := len(leadingWhitespace(line))
		if 0 > width || lead < width {
			width = lead
		}
	}
	if 0 >= width {
		return strings.Join(lines, "\n")
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) >= width {
			out = append(out, line[width:])
			continue
		}
		out = append(out, strings.TrimLeft(line, " \t"))
	}
	return strings.Join(out, "\n")
}

func (e *emitter) textItems(nodes []node, sc *scope) ([]textItem, error) {
	items := make([]textItem, 0, len(nodes))
	pending := ""

	flush := func() {
		if "" == strings.TrimSpace(pending) {
			pending = ""
			return
		}
		lines := strings.Split(pending, "\n")
		for 0 < len(lines) && "" == strings.TrimSpace(lines[0]) {
			lines = lines[1:]
		}
		for 0 < len(lines) && "" == strings.TrimSpace(lines[len(lines)-1]) {
			lines = lines[:len(lines)-1]
		}
		if 0 < len(lines) {
			items = append(items, textItem{lines: lines})
		}
		pending = ""
	}

	for i, n := range nodes {
		switch cur := n.(type) {
		case *textNode:
			pending += trimTextInline(cur.raw, prevNode(nodes, i), nextNode(nodes, i))
		case *actionNode:
			if isAssignment(cur.body) {
				e.bindVariable(cur, sc)
				continue
			}
			if "" == strings.TrimSpace(cur.body) || strings.HasPrefix(cur.body, "/*") {
				continue
			}
			expr, err := e.compileActionBody(cur.body, sc)
			if nil != err {
				return nil, err
			}
			pending += "${" + expr + "}"
		case *blockNode:
			directive, err := e.textDirective(cur, sc)
			if nil != err {
				return nil, err
			}
			if cur.ownLine {
				pending = strings.TrimRight(pending, "\n")
				flush()
				items = append(items, textItem{directive: directive})
				continue
			}
			head, rest := splitLastLine(pending)
			pending = head
			flush()
			items = append(items, textItem{directive: inlineTextItem(rest, directive)})
		default:
			return nil, unsupported("nested define inside a text block")
		}
	}
	flush()
	return items, nil
}

func splitLastLine(pending string) (string, string) {
	idx := strings.LastIndex(pending, "\n")
	if 0 > idx {
		return "", pending
	}
	return pending[:idx+1], pending[idx+1:]
}

func inlineTextItem(prefix string, directive []string) []string {
	out := []string{"$text:"}
	if "" != prefix {
		out = append(out, "  - "+quoteYAML(prefix))
	}
	for j, line := range directive {
		lead := "    "
		if 0 == j {
			lead = "  - "
		}
		out = append(out, lead+line)
	}
	return append(out, "$join: \"\"")
}

func (e *emitter) textDirective(blk *blockNode, sc *scope) ([]string, error) {
	switch blk.kind {
	case "range":
		return e.textEach(blk, sc)
	case "if":
		return e.textIf(blk, sc)
	case "with":
		return e.textWith(blk, sc)
	}
	return nil, unsupported("%s inside a text block", blk.kind)
}

func (e *emitter) textEach(blk *blockNode, sc *scope) ([]string, error) {
	decls, collection := splitRangeDecl(blk.branches[0].cond)
	expr, err := compileExpression(collection, sc)
	if nil != err {
		return nil, err
	}
	e.loopSeq++
	loopVar := "item" + strconv.Itoa(e.loopSeq)
	inner := sc.child(loopVar)
	switch len(decls) {
	case 1:
		inner.set(decls[0], loopVar)
	case 2:
		inner.set(decls[0], loopVar+".key")
		inner.set(decls[1], loopVar+".value")
		inner.dot = loopVar + ".value"
	}
	items, err := e.textItems(trimBranchEdges(blk.branches[0]), inner)
	if nil != err {
		return nil, err
	}
	out := []string{"$each: ${" + expr + "}", "$as: " + loopVar, "$yield:"}
	return append(out, nestItems(items, "  ")...), nil
}

func (e *emitter) textIf(blk *blockNode, sc *scope) ([]string, error) {
	cond, err := compileExpression(blk.branches[0].cond, sc)
	if nil != err {
		return nil, err
	}
	thenItems, err := e.textItems(trimBranchEdges(blk.branches[0]), sc.child(""))
	if nil != err {
		return nil, err
	}
	out := []string{"$if: ${" + cond + "}", "$then:"}
	out = append(out, nestItems(thenItems, "  ")...)
	if 1 == len(blk.branches) {
		return out, nil
	}
	rest, err := e.textBranches(blk, 1, sc)
	if nil != err {
		return nil, err
	}
	out = append(out, "$else:")
	for _, line := range rest {
		out = append(out, "  "+line)
	}
	return out, nil
}

func (e *emitter) textBranches(blk *blockNode, index int, sc *scope) ([]string, error) {
	br := blk.branches[index]
	items, err := e.textItems(trimBranchEdges(br), sc.child(""))
	if nil != err {
		return nil, err
	}
	if "" == br.cond {
		return nestItems(items, "")[0:], nil
	}
	cond, err := compileExpression(br.cond, sc)
	if nil != err {
		return nil, err
	}
	out := []string{"$if: ${" + cond + "}", "$then:"}
	out = append(out, nestItems(items, "  ")...)
	if index+1 >= len(blk.branches) {
		return out, nil
	}
	rest, err := e.textBranches(blk, index+1, sc)
	if nil != err {
		return nil, err
	}
	out = append(out, "$else:")
	for _, line := range rest {
		out = append(out, "  "+line)
	}
	return out, nil
}

func (e *emitter) textWith(blk *blockNode, sc *scope) ([]string, error) {
	target, err := compileExpression(blk.branches[0].cond, sc)
	if nil != err {
		return nil, err
	}
	items, err := e.textItems(trimBranchEdges(blk.branches[0]), sc.child(target))
	if nil != err {
		return nil, err
	}
	out := []string{"$if: ${" + target + "}", "$then:"}
	return append(out, nestItems(items, "  ")...), nil
}

func nestItems(items []textItem, indent string) []string {
	out := make([]string, 0, len(items)*2)
	for _, item := range items {
		if 0 < len(item.directive) {
			for j, line := range item.directive {
				prefix := indent + "    "
				if 0 == j {
					prefix = indent + "  - "
				}
				out = append(out, prefix+line)
			}
			continue
		}
		for _, line := range item.lines {
			out = append(out, indent+"  - "+quoteYAML(line))
		}
	}
	return out
}

func quoteYAML(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\t", `\t`)
	return `"` + escaped + `"`
}
