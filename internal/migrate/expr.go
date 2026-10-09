package migrate

import (
	"fmt"
	"strconv"
	"strings"
	"text/template/parse"

	"github.com/Masterminds/sprig/v3"
	"github.com/ebogdum/keramos/v3/internal/engine"
)

type scope struct {
	dot         string
	vars        map[string]string
	parent      *scope
	tmplName    string
	basePath    string
	valuePrefix string
	chartName   string
}

func newScope() *scope {
	return &scope{dot: "", vars: map[string]string{}}
}

func (s *scope) prefix() string {
	for cur := s; nil != cur; cur = cur.parent {
		if "" != cur.valuePrefix {
			return cur.valuePrefix
		}
	}
	return ""
}

func (s *scope) chart() string {
	for cur := s; nil != cur; cur = cur.parent {
		if "" != cur.chartName {
			return cur.chartName
		}
	}
	return ""
}

func (s *scope) templateName() string {
	for cur := s; nil != cur; cur = cur.parent {
		if "" != cur.tmplName {
			return cur.tmplName
		}
	}
	return ""
}

func (s *scope) templateBasePath() string {
	for cur := s; nil != cur; cur = cur.parent {
		if "" != cur.basePath {
			return cur.basePath
		}
	}
	return ""
}

func (s *scope) child(dot string) *scope {
	return &scope{dot: dot, vars: map[string]string{}, parent: s}
}

func (s *scope) lookup(name string) (string, bool) {
	for cur := s; nil != cur; cur = cur.parent {
		if v, ok := cur.vars[name]; ok {
			return v, true
		}
	}
	return "", false
}

func (s *scope) set(name, expr string) {
	s.vars[name] = expr
}

func (s *scope) setOuter(name, expr string) {
	for cur := s; nil != cur; cur = cur.parent {
		if _, ok := cur.vars[name]; ok {
			cur.vars[name] = expr
			return
		}
	}
	s.vars[name] = expr
}

func (s *scope) dotExpr() string {
	for cur := s; nil != cur; cur = cur.parent {
		if "" != cur.dot {
			return cur.dot
		}
	}
	return ""
}

var parseFuncs = func() map[string]any {
	fm := map[string]any{}
	stub := func() string { return "" }
	for name := range sprig.TxtFuncMap() {
		fm[name] = stub
	}
	for _, name := range []string{
		"include", "tpl", "required", "fail", "lookup", "toYaml", "fromYaml",
		"toToml", "fromYamlArray", "toYamlPretty", "mustToYaml", "getHostByPort",
		"and", "call", "html", "index", "slice", "js", "len", "not", "or",
		"print", "printf", "println", "urlquery", "eq", "ge", "gt", "le", "lt", "ne",
	} {
		fm[name] = stub
	}
	return fm
}()

type unsupportedError struct {
	reason string
}

func (e unsupportedError) Error() string { return e.reason }

func unsupported(format string, args ...any) error {
	return unsupportedError{reason: fmt.Sprintf(format, args...)}
}

func parsePipeline(src string, sc *scope) (*parse.PipeNode, error) {
	prelude, offset := varPrelude(sc)
	trees, err := parse.Parse("expr", prelude+"{{"+src+"}}", "{{", "}}", parseFuncs)
	if nil != err {
		return nil, unsupported("cannot parse %q: %v", src, err)
	}
	tree, ok := trees["expr"]
	if !ok || nil == tree.Root || offset >= len(tree.Root.Nodes) {
		return nil, unsupported("empty expression %q", src)
	}
	action, ok := tree.Root.Nodes[offset].(*parse.ActionNode)
	if !ok {
		return nil, unsupported("not an action: %q", src)
	}
	return action.Pipe, nil
}

func varPrelude(sc *scope) (string, int) {
	names := map[string]bool{}
	for cur := sc; nil != cur; cur = cur.parent {
		for name := range cur.vars {
			names[name] = true
		}
	}
	if 0 == len(names) {
		return "", 0
	}
	var b strings.Builder
	for name := range names {
		b.WriteString("{{ " + name + " := 1 }}")
	}
	return b.String(), len(names)
}

func compileExpression(src string, sc *scope) (string, error) {
	if "" == strings.TrimSpace(src) {
		return "", unsupported("empty expression")
	}
	pipe, err := parsePipeline(src, sc)
	if nil != err {
		return "", err
	}
	return compilePipe(pipe, sc)
}

func compilePipe(pipe *parse.PipeNode, sc *scope) (string, error) {
	if nil == pipe || 0 == len(pipe.Cmds) {
		return "", unsupported("empty pipeline")
	}
	out, err := compileCommand(pipe.Cmds[0], sc)
	if nil != err {
		return "", err
	}
	for _, cmd := range pipe.Cmds[1:] {
		seg, err := compilePipedCommand(cmd, sc)
		if nil != err {
			return "", err
		}
		out = out + " | " + seg
	}
	return out, nil
}

func compilePipedCommand(cmd *parse.CommandNode, sc *scope) (string, error) {
	if 0 == len(cmd.Args) {
		return "", unsupported("empty command")
	}
	name, err := callTargetName(cmd.Args[0])
	if nil != err {
		return "", err
	}
	args, err := compileArgs(cmd.Args[1:], sc)
	if nil != err {
		return "", err
	}
	if "include" == name || "tpl" == name {
		return callWithLiteralContext(name, args)
	}
	if _, known := engine.HelmSubjectPosition(name); !known {
		return "", unsupported("unsupported function %q", name)
	}
	if 0 == len(args) {
		return name, nil
	}
	return name + " " + strings.Join(args, " "), nil
}

func compileCommand(cmd *parse.CommandNode, sc *scope) (string, error) {
	if 0 == len(cmd.Args) {
		return "", unsupported("empty command")
	}
	if 1 == len(cmd.Args) {
		if _, isIdent := cmd.Args[0].(*parse.IdentifierNode); !isIdent {
			return compileArg(cmd.Args[0], sc)
		}
	}
	name, err := callTargetName(cmd.Args[0])
	if nil != err {
		return "", err
	}
	args, err := compileArgs(cmd.Args[1:], sc)
	if nil != err {
		return "", err
	}
	return compileCall(name, args)
}

func compileArgs(nodes []parse.Node, sc *scope) ([]string, error) {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		compiled, err := compileArg(n, sc)
		if nil != err {
			return nil, err
		}
		out = append(out, compiled)
	}
	return out, nil
}

func callTargetName(n parse.Node) (string, error) {
	switch node := n.(type) {
	case *parse.IdentifierNode:
		return helmFuncName(node.Ident), nil
	case *parse.FieldNode:
		return builtinObjectCall(node.Ident)
	case *parse.ChainNode:
		if field, ok := node.Node.(*parse.FieldNode); ok {
			return builtinObjectCall(append(append([]string{}, field.Ident...), node.Field...))
		}
		if variable, ok := node.Node.(*parse.VariableNode); ok {
			path := append(append([]string{}, variable.Ident...), node.Field...)
			return builtinObjectCall(trimVariableMarker(path))
		}
	case *parse.VariableNode:
		return builtinObjectCall(trimVariableMarker(node.Ident))
	}
	return "", unsupported("not a callable: %s", n.String())
}

func trimVariableMarker(path []string) []string {
	out := make([]string, 0, len(path))
	for _, segment := range path {
		out = append(out, strings.TrimPrefix(segment, "$"))
	}
	return out
}

func builtinObjectCall(path []string) (string, error) {
	for i, segment := range path {
		switch segment {
		case "Files", "files", "Capabilities", "capabilities":
			if i+1 < len(path) {
				return strings.ToUpper(segment[:1]) + segment[1:] + "." + strings.Join(path[i+1:], "."), nil
			}
		}
	}
	return "", unsupported("not a callable: .%s", strings.Join(path, "."))
}

func compileCall(name string, args []string) (string, error) {
	if "include" == name || "tpl" == name {
		return callWithLiteralContext(name, args)
	}
	pos, known := engine.HelmSubjectPosition(name)
	if !known {
		return "", unsupported("unsupported function %q", name)
	}
	switch {
	case 0 == len(args):
		return name, nil
	case -2 == pos:
		return name + " " + strings.Join(args, " "), nil
	case -1 == pos:
		return pipeForm(args[len(args)-1], name, args[:len(args)-1]), nil
	case pos < len(args):
		rest := make([]string, 0, len(args)-1)
		rest = append(rest, args[:pos]...)
		rest = append(rest, args[pos+1:]...)
		return pipeForm(args[pos], name, rest), nil
	default:
		return name + " " + strings.Join(args, " "), nil
	}
}

func callWithLiteralContext(name string, args []string) (string, error) {
	if 0 == len(args) {
		return "", unsupported("%s needs a template name", name)
	}
	out := name + " " + args[0]
	if 1 < len(args) && !isRootContext(args[1]) {
		out += " " + args[1]
	}
	return out, nil
}

func isRootContext(arg string) bool {
	switch arg {
	case "", "values", "root":
		return true
	}
	return false
}

func pipeForm(subject, name string, rest []string) string {
	if !isSimplePath(subject) && !isLiteral(subject) && !strings.HasPrefix(subject, "(") {
		subject = "(" + subject + ")"
	}
	if 0 == len(rest) {
		return subject + " | " + name
	}
	return subject + " | " + name + " " + strings.Join(rest, " ")
}

func isLiteral(s string) bool {
	if "" == s {
		return false
	}
	if strings.ContainsAny(s, " \t|") && ('\'' == s[0] || '"' == s[0]) {
		return false
	}
	if '\'' == s[0] || '"' == s[0] {
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); nil == err {
		return true
	}
	return "true" == s || "false" == s || "null" == s
}

func helmFuncName(name string) string {
	switch name {
	case "b64enc":
		return "b64encode"
	case "b64dec":
		return "b64decode"
	}
	return name
}

func compileArg(n parse.Node, sc *scope) (string, error) {
	switch node := n.(type) {
	case *parse.StringNode:
		return quoteLiteral(node.Text), nil
	case *parse.NumberNode:
		return node.Text, nil
	case *parse.BoolNode:
		return strconv.FormatBool(node.True), nil
	case *parse.NilNode:
		return "null", nil
	case *parse.DotNode:
		return dotReference(sc, nil)
	case *parse.FieldNode:
		return dotReference(sc, node.Ident)
	case *parse.VariableNode:
		return variableReference(node, sc)
	case *parse.PipeNode:
		inner, err := compilePipe(node, sc)
		if nil != err {
			return "", err
		}
		return "(" + inner + ")", nil
	case *parse.ChainNode:
		return compileChain(node, sc)
	case *parse.IdentifierNode:
		switch node.Ident {
		case "dict":
			return "('{}' | fromYaml)", nil
		case "list", "tuple":
			return "('[]' | fromYamlArray)", nil
		}
		return "", unsupported("bare function reference %q", node.Ident)
	}
	return "", unsupported("unsupported expression node %s", n.String())
}

func compileChain(node *parse.ChainNode, sc *scope) (string, error) {
	base, err := compileArg(node.Node, sc)
	if nil != err {
		return "", err
	}
	return pathAccess(base, node.Field), nil
}

func pathAccess(base string, fields []string) string {
	if 0 == len(fields) {
		return base
	}
	if isSimplePath(base) {
		return base + "." + strings.Join(fields, ".")
	}
	expr := strings.TrimSuffix(strings.TrimPrefix(base, "("), ")")
	for _, f := range fields {
		expr += " | get " + quoteLiteral(f)
	}
	return "(" + expr + ")"
}

func variableReference(node *parse.VariableNode, sc *scope) (string, error) {
	name := node.Ident[0]
	rest := node.Ident[1:]

	if "$" == name {
		return rootReference(rest, sc)
	}
	bound, ok := sc.lookup(name)
	if !ok {
		return "", unsupported("unknown variable %s", name)
	}
	if 0 == len(rest) && !isSimplePath(bound) && !isLiteral(bound) && !strings.HasPrefix(bound, "(") {
		return "(" + bound + ")", nil
	}
	return pathAccess(bound, rest), nil
}

func isSimplePath(expr string) bool {
	if "" == expr {
		return false
	}
	for _, ch := range expr {
		if ' ' == ch || '|' == ch || '(' == ch || ')' == ch || '"' == ch || '\'' == ch {
			return false
		}
	}
	return true
}

func rootReference(fields []string, sc *scope) (string, error) {
	if 0 == len(fields) {
		return "values", nil
	}
	return mapRootPath(fields, sc)
}

func dotReference(sc *scope, fields []string) (string, error) {
	dot := sc.dotExpr()
	if 0 == len(fields) {
		if "" == dot {
			return "values", nil
		}
		return dot, nil
	}
	if "" == dot {
		return mapRootPath(fields, sc)
	}
	return pathAccess(dot, fields), nil
}

var releaseFields = map[string]string{
	"Service":   "'keramos'",
	"Name":      "release.name",
	"Namespace": "release.namespace",
	"Revision":  "release.revision",
	"IsInstall": "release.isInstall",
	"IsUpgrade": "release.isUpgrade",
}

var chartFields = map[string]string{
	"Name":       "package.name",
	"Version":    "package.version",
	"AppVersion": "package.appVersion",
}

func mapRootPath(fields []string, sc *scope) (string, error) {
	head := fields[0]
	rest := fields[1:]
	switch head {
	case "Values":
		prefix := sc.prefix()
		if 0 == len(rest) {
			if "" == prefix {
				return "values", nil
			}
			return "values." + prefix, nil
		}
		if "" == prefix || "global" == rest[0] {
			return "values." + strings.Join(rest, "."), nil
		}
		return "values." + prefix + "." + strings.Join(rest, "."), nil
	case "Release":
		if 0 == len(rest) {
			return "release", nil
		}
		if mapped, ok := releaseFields[rest[0]]; ok && 1 == len(rest) {
			return mapped, nil
		}
		return "release." + lowerFirst(strings.Join(rest, ".")), nil
	case "Chart":
		if 0 == len(rest) {
			return "package", nil
		}
		if "Name" == rest[0] && 1 == len(rest) && "" != sc.prefix() {
			return quoteLiteral(sc.chart()), nil
		}
		if mapped, ok := chartFields[rest[0]]; ok && 1 == len(rest) {
			return mapped, nil
		}
		return "package." + lowerFirst(strings.Join(rest, ".")), nil
	case "Capabilities":
		if 0 == len(rest) {
			return "capabilities", nil
		}
		if "KubeVersion" == rest[0] {
			if 1 == len(rest) {
				return "capabilities.kubeVersion.Version", nil
			}
			return "capabilities.kubeVersion." + strings.Join(rest[1:], "."), nil
		}
		return "capabilities." + lowerFirst(strings.Join(rest, ".")), nil
	case "Template":
		if 0 < len(rest) && "BasePath" == rest[0] {
			return quoteLiteral(sc.templateBasePath()), nil
		}
		return quoteLiteral(sc.templateName()), nil
	case "Subcharts":
		if 0 == len(rest) {
			return "values", nil
		}
		return "values." + strings.Join(rest, "."), nil
	}
	return "values." + strings.Join(fields, "."), nil
}

func lowerFirst(s string) string {
	if "" == s {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func quoteLiteral(s string) string {
	if !strings.ContainsAny(s, "'\n\r\t\\\"") {
		return "'" + s + "'"
	}
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	escaped = strings.ReplaceAll(escaped, "\r", `\r`)
	escaped = strings.ReplaceAll(escaped, "\t", `\t`)
	return `"` + escaped + `"`
}
