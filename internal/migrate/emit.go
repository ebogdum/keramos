package migrate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type emitter struct {
	branchCond string
	file       string
	result     *MigrateResult
	reviews    []ReviewItem
	helmKept   []string
	textOnly   bool
	loopSeq    int

	lets  []letBinding
	buf   strings.Builder
	line  int
	spans structurer
}

func newEmitter(file string, result *MigrateResult) *emitter {
	return &emitter{file: file, result: result, line: 1}
}

func (e *emitter) write(text string) {
	e.buf.WriteString(text)
	e.line += strings.Count(text, "\n")
}

func (e *emitter) atLineStart() bool {
	text := e.buf.String()
	idx := strings.LastIndexByte(text, '\n')
	return "" == strings.TrimSpace(text[idx+1:])
}

func (e *emitter) review(line int, reason, original string) {
	e.reviews = append(e.reviews, ReviewItem{
		File: e.file, Line: line, Reason: reason, Original: original,
	})
}

func (e *emitter) flushReviews() {
	e.result.ManualReview = append(e.result.ManualReview, e.reviews...)
}

func (e *emitter) emit(nodes []node, sc *scope) {
	for i, n := range nodes {
		switch cur := n.(type) {
		case *textNode:
			e.write(trimTextFor(cur.raw, prevNode(nodes, i), nextNode(nodes, i)))
		case *actionNode:
			e.emitAction(cur, sc, nextNode(nodes, i))
		case *blockNode:
			e.emitBlock(cur, sc)
		case *defineNode:
			e.review(cur.line, "nested define is not converted", cur.name)
		}
	}
}

func prevNode(nodes []node, i int) node {
	if 0 == i {
		return nil
	}
	return nodes[i-1]
}

func nextNode(nodes []node, i int) node {
	if i+1 >= len(nodes) {
		return nil
	}
	return nodes[i+1]
}

func trimTextFor(raw string, prev, next node) string {
	if trimsRight(prev) {
		raw = strings.TrimLeft(raw, " \t\r\n")
	} else if ownLineOf(prev) {
		raw = dropLeadingLine(raw)
	}
	if trimsLeft(next) {
		raw = strings.TrimRight(raw, " \t\r\n")
	} else if ownLineOf(next) {
		raw = dropTrailingIndent(raw)
	}
	return raw
}

func dropLeadingLine(raw string) string {
	idx := strings.IndexByte(raw, '\n')
	if 0 > idx {
		return raw
	}
	if "" != strings.TrimSpace(raw[:idx]) {
		return raw
	}
	return raw[idx:]
}

func dropTrailingIndent(raw string) string {
	idx := strings.LastIndexByte(raw, '\n')
	if 0 > idx {
		if "" == strings.TrimSpace(raw) {
			return ""
		}
		return raw
	}
	if "" == strings.TrimSpace(raw[idx+1:]) {
		return raw[:idx+1]
	}
	return raw
}

func ownLineOf(n node) bool {
	switch v := n.(type) {
	case *actionNode:
		return v.ownLine
	case *blockNode:
		return v.ownLine
	case *defineNode:
		return true
	}
	return false
}

func trimsLeft(n node) bool {
	switch v := n.(type) {
	case *actionNode:
		return v.trimL && !v.ownLine
	case *blockNode:
		return v.openTrimL && !v.ownLine
	}
	return false
}

func trimsRight(n node) bool {
	switch v := n.(type) {
	case *actionNode:
		return v.trimR && !v.ownLine
	case *blockNode:
		return v.endTrimR && !v.ownLine
	}
	return false
}

var reNindent = regexp.MustCompile(`\s*\|\s*(n?indent)\s+(\d+)\s*$`)

func (e *emitter) emitAction(act *actionNode, sc *scope, next node) {
	body := act.body
	if "" == body {
		return
	}
	if strings.HasPrefix(body, "/*") || strings.HasSuffix(body, "*/") {
		return
	}
	if "template" == directiveWord(body) {
		e.emitTemplateCall(act, sc)
		return
	}
	if isAssignment(body) {
		e.bindVariable(act, sc)
		return
	}
	if act.ownLine && !e.textOnly && !e.inBlockScalar(len(act.indent)) {
		e.emitStructuralAction(act, sc)
		return
	}
	if !e.textOnly && e.emitKeyValueAction(act, sc, next) {
		return
	}
	if act.ownLine && e.atLineStart() {
		e.write(act.indent)
	}
	expr, err := compileExpression(body, sc)
	if nil != err {
		e.fenceHelm(act.raw, act.indent, err.Error())
		return
	}
	e.write("${" + expr + "}")
}

func (e *emitter) compileActionBody(body string, sc *scope) (string, error) {
	if "template" == directiveWord(body) {
		rest := directiveArgument(body, "template")
		name, ctx := splitTemplateArgs(rest)
		if "" == name {
			return "", unsupported("template call without a name")
		}
		call := "include " + quoteLiteral(name)
		if arg := e.contextArg(ctx, sc); "" != arg {
			call += " " + arg
		}
		return call, nil
	}
	return compileExpression(body, sc)
}

func isAssignment(body string) bool {
	idx := strings.Index(body, "=")
	if 0 > idx {
		return false
	}
	if idx+1 < len(body) && '=' == body[idx+1] {
		return false
	}
	head := strings.TrimSpace(strings.TrimSuffix(body[:idx], ":"))
	return strings.HasPrefix(head, "$") && !strings.ContainsAny(head, "()|\"' ")
}

func (e *emitter) bindVariable(act *actionNode, sc *scope) {
	name, rhs, reassign := splitAssignment(act.body)
	expr, err := compileExpression(rhs, sc)
	if nil != err {
		e.review(act.line, err.Error(), "{{ "+act.body+" }}")
		sc.set(name, "null")
		return
	}
	if reassign && "" != e.branchCond {
		if previous, ok := sc.lookup(name); ok {
			expr = wrapArg(e.branchCond) + " | ternary " + wrapArg(expr) + " " + wrapArg(previous)
		}
	}
	sc.setOuter(name, expr)
}

func splitAssignment(body string) (string, string, bool) {
	if idx := strings.Index(body, ":="); 0 <= idx {
		return strings.TrimSpace(body[:idx]), strings.TrimSpace(body[idx+2:]), false
	}
	idx := strings.Index(body, "=")
	return strings.TrimSpace(body[:idx]), strings.TrimSpace(body[idx+1:]), true
}

func (e *emitter) emitTemplateCall(act *actionNode, sc *scope) {
	rest := directiveArgument(act.body, "template")
	name, ctx := splitTemplateArgs(rest)
	if "" == name {
		e.review(act.line, "template call without a name", "{{ "+act.body+" }}")
		return
	}
	call := "include " + quoteLiteral(name)
	if arg := e.contextArg(ctx, sc); "" != arg {
		call += " " + arg
	}
	if act.ownLine && !e.textOnly {
		e.recordAction(&actionSpan{line: e.line, column: len(act.indent) + 1, partial: name, expr: call + " | fromYaml"})
		return
	}
	e.write("${" + call + "}")
}

func (e *emitter) contextArg(ctx string, sc *scope) string {
	ctx = strings.TrimSpace(ctx)
	if "" == ctx || "." == ctx || "$" == ctx {
		return ""
	}
	expr, err := compileExpression(ctx, sc)
	if nil != err {
		return ""
	}
	if isRootContext(expr) {
		return ""
	}
	return expr
}

func splitTemplateArgs(rest string) (string, string) {
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, `"`) && !strings.HasPrefix(rest, "'") {
		return "", ""
	}
	quote := rest[0]
	end := strings.IndexByte(rest[1:], quote)
	if 0 > end {
		return "", ""
	}
	return rest[1 : end+1], strings.TrimSpace(rest[end+2:])
}

func (e *emitter) recordAction(span *actionSpan) {
	e.spans.actions = append(e.spans.actions, span)
}

func (e *emitter) emitStructuralAction(act *actionNode, sc *scope) {
	body, _ := splitNindent(act.body)
	expr, err := compileExpression(body, sc)
	if nil != err {
		e.keepHelm(act.raw, act.indent, err.Error())
		return
	}
	if name, ok := plainIncludeName(expr); ok {
		e.recordAction(&actionSpan{line: e.line, column: actionColumn(act), partial: name, expr: expr + " | fromYaml"})
		return
	}
	e.recordAction(&actionSpan{line: e.line, column: actionColumn(act), expr: structuralExpr(expr)})
}

func actionColumn(act *actionNode) int {
	if _, width := splitNindent(act.body); 0 < width {
		return width + 1
	}
	return len(act.indent) + 1
}

func plainIncludeName(expr string) (string, bool) {
	if !strings.HasPrefix(expr, "include ") || strings.ContainsAny(expr, "|") {
		return "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(expr, "include "))
	if !strings.HasPrefix(rest, "'") {
		return "", false
	}
	end := strings.IndexByte(rest[1:], '\'')
	if 0 > end {
		return "", false
	}
	if "" != strings.TrimSpace(rest[end+2:]) {
		return "", false
	}
	return rest[1 : end+1], true
}

func structuralExpr(expr string) string {
	expr = dropStructuralToYaml(expr)
	if strings.HasSuffix(expr, "| fromYaml") || strings.HasSuffix(expr, "| fromYamlArray") {
		return expr
	}
	if rendersText(expr) {
		return expr + " | fromYaml"
	}
	return expr
}

func rendersText(expr string) bool {
	for _, fn := range []string{"include ", "tpl ", "| include", "| tpl", "toJson", "toYaml"} {
		if strings.Contains(expr, fn) {
			return true
		}
	}
	return false
}

func (e *emitter) emitKeyValueAction(act *actionNode, sc *scope, next node) bool {
	if !reNindent.MatchString(act.body) {
		return false
	}
	if !endsWithMapKey(e.buf.String()) || !startsNewLine(next) {
		return false
	}
	body, _ := splitNindent(act.body)
	expr, err := compileExpression(body, sc)
	if nil != err {
		return false
	}
	e.write(" ${" + structuralExpr(expr) + "}")
	return true
}

func endsWithMapKey(text string) bool {
	idx := strings.LastIndexByte(text, '\n')
	line := strings.TrimRight(text[idx+1:], " \t")
	if "" == line || !strings.HasSuffix(line, ":") {
		return false
	}
	return "" != strings.TrimSpace(strings.TrimSuffix(line, ":"))
}

func startsNewLine(next node) bool {
	if nil == next {
		return true
	}
	t, isText := next.(*textNode)
	if !isText {
		return false
	}
	trimmed := strings.TrimLeft(t.raw, " \t")
	return strings.HasPrefix(trimmed, "\n") || "" == trimmed
}

func splitNindent(body string) (string, int) {
	m := reNindent.FindStringSubmatch(body)
	if nil == m {
		return body, 0
	}
	width, err := strconv.Atoi(m[2])
	if nil != err {
		width = 0
	}
	return reNindent.ReplaceAllString(body, ""), width
}

func dropStructuralToYaml(expr string) string {
	for _, suffix := range []string{" | toYaml", " | toYamlPretty"} {
		if strings.HasSuffix(expr, suffix) {
			return strings.TrimSuffix(expr, suffix)
		}
	}
	return expr
}

func (e *emitter) inBlockScalar(indent int) bool {
	lines := strings.Split(e.buf.String(), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if "" == strings.TrimSpace(line) {
			continue
		}
		if len(leadingWhitespace(line)) >= indent {
			continue
		}
		trimmed := strings.TrimRight(line, " \t")
		for _, marker := range []string{"|", "|-", "|+", ">", ">-", ">+"} {
			if strings.HasSuffix(trimmed, ": "+marker) || strings.HasSuffix(trimmed, ":"+marker) {
				return true
			}
		}
		return false
	}
	return false
}

func leadingWhitespace(line string) string {
	for i := 0; i < len(line); i++ {
		if ' ' != line[i] && '\t' != line[i] {
			return line[:i]
		}
	}
	return line
}

func (e *emitter) emitBlock(blk *blockNode, sc *scope) {
	if !blk.ownLine && !e.textOnly && endsWithMapKey(e.buf.String()) && spansLines(blk) {
		e.write("\n")
		switch blk.kind {
		case "if":
			e.emitIfBlock(blk, sc)
		case "with":
			e.emitWithBlock(blk, sc)
		case "range":
			e.emitRangeBlock(blk, sc)
		}
		return
	}
	if blk.ownLine && !e.textOnly && e.inBlockScalar(len(blk.indent)) {
		e.emitScalarBlock(blk, sc)
		return
	}
	if !blk.ownLine || e.textOnly {
		e.emitInlineBlock(blk, sc)
		return
	}
	switch blk.kind {
	case "if":
		e.emitIfBlock(blk, sc)
	case "with":
		e.emitWithBlock(blk, sc)
	case "range":
		e.emitRangeBlock(blk, sc)
	}
}

func (e *emitter) keepHelm(raw, indent, reason string) {
	e.helmKept = append(e.helmKept, reason)
}

func (e *emitter) fenceHelm(raw, indent, reason string) {
	e.keepHelm(raw, indent, reason)
}

func (e *emitter) emitScalarBlock(blk *blockNode, sc *scope) {
	expr, err := e.inlineBlockExpr(blk, sc)
	if nil != err {
		e.keepHelm(blk.raw, blk.indent, fmt.Sprintf("%s block on line %d inside a literal block: %s", blk.kind, blk.line, err.Error()))
		_ = expr
		return
	}
	if e.atLineStart() {
		e.write(blk.indent)
	}
	e.write("${" + expr + "}")
}

func (e *emitter) emitIfBlock(blk *blockNode, sc *scope) {
	span := &blockSpan{kind: "if", line: blk.line, column: len(blk.indent) + 1}
	for _, br := range blk.branches {
		cond := ""
		if "" != br.cond {
			compiled, err := compileExpression(br.cond, sc)
			if nil != err {
				e.review(blk.line, err.Error(), "{{ if "+br.cond+" }}")
			}
			cond = compiled
		}
		span.branches = append(span.branches, branchSpan{cond: cond, span: e.emitBranch(br.body, sc.child(""))})
	}
	e.spans.blocks = append(e.spans.blocks, span)
}

func (e *emitter) emitWithBlock(blk *blockNode, sc *scope) {
	main := blk.branches[0]
	expr, err := compileExpression(main.cond, sc)
	if nil != err {
		e.review(blk.line, err.Error(), "{{ with "+main.cond+" }}")
	}
	span := &blockSpan{kind: "if", line: blk.line, column: len(blk.indent) + 1}
	span.branches = append(span.branches, branchSpan{cond: expr, span: e.emitBranch(main.body, sc.child(expr))})
	for _, br := range blk.branches[1:] {
		span.branches = append(span.branches, branchSpan{cond: "", span: e.emitBranch(br.body, sc.child(""))})
	}
	e.spans.blocks = append(e.spans.blocks, span)
}

func (e *emitter) emitRangeBlock(blk *blockNode, sc *scope) {
	main := blk.branches[0]
	decls, collection := splitRangeDecl(main.cond)
	expr, err := compileExpression(collection, sc)
	if nil != err {
		e.review(blk.line, err.Error(), "{{ range "+main.cond+" }}")
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
	span := &blockSpan{kind: "each", expr: expr, loopVar: loopVar, line: blk.line, column: len(blk.indent) + 1}
	span.branches = append(span.branches, branchSpan{span: e.emitBranch(main.body, inner)})
	for _, br := range blk.branches[1:] {
		span.branches = append(span.branches, branchSpan{cond: "", span: e.emitBranch(br.body, sc.child(""))})
	}
	e.spans.blocks = append(e.spans.blocks, span)
}

func (e *emitter) emitBranch(body []node, sc *scope) lineRange {
	start := e.line
	e.emit(body, sc)
	end := e.line
	if !e.atLineStart() {
		end++
	}
	return lineRange{start: start, end: end}
}

func splitRangeDecl(cond string) ([]string, string) {
	idx := strings.Index(cond, ":=")
	if 0 > idx {
		return nil, cond
	}
	head := strings.TrimSpace(cond[:idx])
	rest := strings.TrimSpace(cond[idx+2:])
	parts := strings.Split(head, ",")
	decls := make([]string, 0, len(parts))
	for _, p := range parts {
		decls = append(decls, strings.TrimSpace(p))
	}
	return decls, rest
}

func (e *emitter) emitInlineBlock(blk *blockNode, sc *scope) {
	expr, err := e.inlineBlockExpr(blk, sc)
	if nil == err {
		e.write("${" + expr + "}")
		return
	}
	e.keepHelm(blk.raw, blk.indent, fmt.Sprintf("%s block on line %d: %s", blk.kind, blk.line, err.Error()))
}

func spansLines(blk *blockNode) bool {
	return strings.Contains(blk.raw, "\n")
}

func inlineSource(blk *blockNode) string {
	if 0 == len(blk.branches) {
		return "{{ " + blk.kind + " }}"
	}
	return "{{ " + blk.kind + " " + blk.branches[0].cond + " }}"
}

func (e *emitter) inlineBlockExpr(blk *blockNode, sc *scope) (string, error) {
	switch blk.kind {
	case "if":
		return e.inlineIfExpr(blk, sc)
	case "with":
		return e.inlineWithExpr(blk, sc)
	case "range":
		return e.inlineRangeExpr(blk, sc)
	}
	return "", unsupported("%s cannot become an expression", blk.kind)
}

func (e *emitter) inlineIfExpr(blk *blockNode, sc *scope) (string, error) {
	branches := blk.branches
	last := len(branches) - 1
	elseExpr := "''"
	if "" == branches[last].cond {
		compiled, err := e.inlineBranchExpr(branches[last], sc)
		if nil != err {
			return "", err
		}
		elseExpr = compiled
		last--
	}
	out := elseExpr
	for i := last; i >= 0; i-- {
		cond, err := compileExpression(branches[i].cond, sc)
		if nil != err {
			return "", err
		}
		thenExpr, err := e.inlineBranchExpr(branches[i], sc)
		if nil != err {
			return "", err
		}
		out = wrapArg(cond) + " | ternary " + wrapArg(thenExpr) + " " + wrapArg(out)
	}
	return out, nil
}

func (e *emitter) inlineWithExpr(blk *blockNode, sc *scope) (string, error) {
	if 1 != len(blk.branches) {
		return "", unsupported("with/else cannot become an expression")
	}
	target, err := compileExpression(blk.branches[0].cond, sc)
	if nil != err {
		return "", err
	}
	body, err := e.inlineBranchExpr(blk.branches[0], sc.child(target))
	if nil != err {
		return "", err
	}
	return wrapArg(target) + " | ternary " + wrapArg(body) + " ''", nil
}

func (e *emitter) inlineRangeExpr(blk *blockNode, sc *scope) (string, error) {
	if 1 != len(blk.branches) {
		return "", unsupported("range/else cannot become an expression")
	}
	decls, collection := splitRangeDecl(blk.branches[0].cond)
	target, err := compileExpression(collection, sc)
	if nil != err {
		return "", err
	}
	separator, field, ok := simpleJoinBody(blk.branches[0].body, decls)
	if !ok {
		return formatEachExpr(blk.branches[0], decls, target)
	}
	if "" != field {
		return wrapArg(target) + " | pluck " + quoteLiteral(field) + " | join " + quoteLiteral(separator), nil
	}
	return wrapArg(target) + " | join " + quoteLiteral(separator), nil
}

func formatEachExpr(br branch, decls []string, target string) (string, error) {
	format := ""
	fields := make([]string, 0, 4)
	for _, n := range trimBranchEdges(br) {
		switch cur := n.(type) {
		case *textNode:
			format += strings.ReplaceAll(cur.raw, "%", "%%")
		case *actionNode:
			field, ok := loopField(cur.body, decls)
			if !ok {
				return "", unsupported("range body is not a simple join")
			}
			format += "%v"
			fields = append(fields, field)
		default:
			return "", unsupported("range body is not a simple join")
		}
	}
	if 0 == len(fields) || strings.Contains(format, "\n") {
		return "", unsupported("range body is not a simple join")
	}
	out := wrapArg(target) + " | formatEach " + quoteLiteral(format)
	for _, field := range fields {
		out += " " + quoteLiteral(field)
	}
	return out + " | join ''", nil
}

func loopField(body string, decls []string) (string, bool) {
	trimmed := strings.TrimSpace(body)
	if "." == trimmed {
		return ".", true
	}
	if strings.HasPrefix(trimmed, ".") && !strings.ContainsAny(trimmed, " |()") {
		return strings.TrimPrefix(trimmed, "."), true
	}
	switch len(decls) {
	case 1:
		if trimmed == decls[0] {
			return ".", true
		}
		if strings.HasPrefix(trimmed, decls[0]+".") {
			return strings.TrimPrefix(trimmed, decls[0]+"."), true
		}
	case 2:
		if trimmed == decls[0] {
			return "key", true
		}
		if trimmed == decls[1] {
			return "value", true
		}
		if strings.HasPrefix(trimmed, decls[1]+".") {
			return "value." + strings.TrimPrefix(trimmed, decls[1]+"."), true
		}
	}
	return "", false
}

func simpleJoinBody(nodes []node, decls []string) (string, string, bool) {
	var separator, field string
	seenAction := false
	for _, n := range nodes {
		switch cur := n.(type) {
		case *textNode:
			if "" == strings.TrimSpace(cur.raw) && "" == separator {
				continue
			}
			if "" != separator && separator != cur.raw {
				return "", "", false
			}
			separator = cur.raw
		case *actionNode:
			if seenAction {
				return "", "", false
			}
			seenAction = true
			body := strings.TrimSpace(cur.body)
			switch {
			case "." == body:
			case strings.HasPrefix(body, "."):
				field = strings.TrimPrefix(body, ".")
			case 1 == len(decls) && body == decls[0]:
			default:
				return "", "", false
			}
		default:
			return "", "", false
		}
	}
	if !seenAction {
		return "", "", false
	}
	if strings.ContainsAny(field, ". |") {
		return "", "", false
	}
	return separator, field, true
}

func (e *emitter) inlineBranchExpr(br branch, sc *scope) (string, error) {
	return e.inlineNodesExpr(trimBranchEdges(br), sc)
}

func trimBranchEdges(br branch) []node {
	nodes := append([]node{}, br.body...)
	if 0 == len(nodes) {
		return nodes
	}
	if br.trimStart {
		if t, ok := nodes[0].(*textNode); ok {
			nodes[0] = &textNode{raw: strings.TrimLeft(t.raw, " \t\r\n")}
		}
	}
	if br.trimEnd {
		last := len(nodes) - 1
		if t, ok := nodes[last].(*textNode); ok {
			nodes[last] = &textNode{raw: strings.TrimRight(t.raw, " \t\r\n")}
		}
	}
	return nodes
}

func (e *emitter) inlineNodesExpr(nodes []node, sc *scope) (string, error) {
	parts := make([]string, 0, len(nodes))
	literal := true
	for i, n := range nodes {
		switch cur := n.(type) {
		case *textNode:
			text := trimBranchText(trimTextInline(cur.raw, prevNode(nodes, i), nextNode(nodes, i)), i, len(nodes))
			if "" == text {
				continue
			}
			parts = append(parts, quoteLiteral(text))
		case *actionNode:
			if isAssignment(cur.body) {
				e.bindVariable(cur, sc)
				continue
			}
			expr, err := e.compileActionBody(cur.body, sc)
			if nil != err {
				return "", err
			}
			parts = append(parts, expr)
			literal = false
		case *blockNode:
			expr, err := e.inlineBlockExpr(cur, sc)
			if nil != err {
				return "", err
			}
			parts = append(parts, expr)
			literal = false
		default:
			return "", unsupported("nested define cannot become an expression")
		}
	}
	switch {
	case 0 == len(parts):
		return "''", nil
	case 1 == len(parts):
		return parts[0], nil
	case literal:
		return quoteLiteral(concatLiterals(parts)), nil
	}
	format := strings.Repeat("%s", len(parts))
	args := make([]string, 0, len(parts))
	for _, p := range parts {
		args = append(args, wrapArg(p))
	}
	return "printf " + quoteLiteral(format) + " " + strings.Join(args, " "), nil
}

func isSingleToken(s string) bool {
	return !strings.ContainsAny(s, " \t|")
}

func trimTextInline(raw string, prev, next node) string {
	if trimsRightInline(prev) {
		raw = strings.TrimLeft(raw, " \t\r\n")
	}
	if trimsLeftInline(next) {
		raw = strings.TrimRight(raw, " \t\r\n")
	}
	return raw
}

func trimsLeftInline(n node) bool {
	switch v := n.(type) {
	case *actionNode:
		return v.trimL
	case *blockNode:
		return v.openTrimL
	}
	return false
}

func trimsRightInline(n node) bool {
	switch v := n.(type) {
	case *actionNode:
		return v.trimR
	case *blockNode:
		return v.endTrimR
	}
	return false
}

func trimBranchText(text string, index, count int) string {
	if 0 == index {
		text = strings.TrimLeft(text, "\n")
	}
	if index == count-1 {
		text = strings.TrimRight(text, "\n")
	}
	return text
}

func concatLiterals(parts []string) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(unquoteLiteral(p))
	}
	return b.String()
}

func unquoteLiteral(s string) string {
	if 2 > len(s) {
		return s
	}
	if ('\'' == s[0] && '\'' == s[len(s)-1]) || ('"' == s[0] && '"' == s[len(s)-1]) {
		return s[1 : len(s)-1]
	}
	return s
}

func wrapArg(expr string) string {
	if isSimplePath(expr) || (isLiteral(expr) && isSingleToken(expr)) {
		return expr
	}
	if isFullyParenthesised(expr) {
		return expr
	}
	return "(" + expr + ")"
}

func isFullyParenthesised(expr string) bool {
	if 2 > len(expr) || '(' != expr[0] || ')' != expr[len(expr)-1] {
		return false
	}
	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
			if 0 == depth && i != len(expr)-1 {
				return false
			}
		}
	}
	return 0 == depth
}
