package migrate

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type templateConverter struct {
	result        *MigrateResult
	partials      map[string]*yaml.Node
	order         []string
	chartName     string
	valuePrefix   string
	lastTextError string
}

func newTemplateConverter(result *MigrateResult) *templateConverter {
	return &templateConverter{result: result, partials: map[string]*yaml.Node{}}
}

func (c *templateConverter) rootScope(filename string) *scope {
	sc := newScope()
	sc.valuePrefix = c.valuePrefix
	sc.chartName = c.chartName
	sc.tmplName = c.chartName + "/templates/" + filename
	sc.basePath = c.chartName + "/templates"
	return sc
}

func (c *templateConverter) addPartial(name string, value *yaml.Node) {
	if _, exists := c.partials[name]; !exists {
		c.order = append(c.order, name)
	}
	c.partials[name] = value
}

func (c *templateConverter) convertFile(content, filename string) string {
	original := content
	content = c.rewriteTextBlocks(content, filename)
	nodes, err := buildNodes(content)
	if nil != err {
		return c.helmFallback(original, filename, err.Error())
	}

	body := c.extractDefines(nodes, filename)
	if isPartialSource(filename) {
		return ""
	}
	if 0 == len(body) {
		if hasTemplateLogic(original) {
			return c.helmFallback(original, filename, "text-only template: "+c.lastTextError)
		}
		return ""
	}
	if doc, ok := c.textOnlyDocument(body, filename); ok {
		return doc
	}

	e := newEmitter(filename, c.result)
	e.emit(body, c.rootScope(filename))
	if 0 < len(e.helmKept) {
		return c.helmFallback(original, filename, e.helmKept[0])
	}
	out, reason := c.finish(e, filename)
	if "" != reason {
		if node, ok := c.textNodeFor(body, filename); ok {
			marshalled, err := yaml.Marshal(node)
			if nil == err {
				return string(marshalled)
			}
		}
		return c.helmFallback(original, filename, reason)
	}
	e.flushReviews()
	return out
}

func isPartialSource(filename string) bool {
	return strings.HasPrefix(filepath.Base(filename), "_")
}

func hasTemplateLogic(content string) bool {
	return strings.Contains(content, "{{")
}

func (c *templateConverter) helmFallback(content, filename, reason string) string {
	c.review(filename, 1, reason, "")
	return ""
}

func (c *templateConverter) textOnlyDocument(body []node, filename string) (string, bool) {
	probe := newEmitter(filename, c.result)
	probe.emit(body, c.rootScope(filename))
	if looksLikeMapping(probe.buf.String()) {
		return "", false
	}
	node, ok := c.textNodeFor(body, filename)
	if !ok {
		return "", false
	}
	out, err := yaml.Marshal(node)
	if nil != err {
		return "", false
	}
	return string(out), true
}

func (c *templateConverter) review(file string, line int, reason, original string) {
	c.result.ManualReview = append(c.result.ManualReview, ReviewItem{
		File: file, Line: line, Reason: reason, Original: original,
	})
}

func (c *templateConverter) finish(e *emitter, filename string) (string, string) {
	skeleton := e.buf.String()
	out, unplaced, err := e.spans.apply(skeleton)
	if nil != err {
		return "", "converted output is not valid YAML: " + err.Error()
	}
	if strings.Contains(out, "{{") {
		return "", "output still contains Helm syntax"
	}
	if 0 < len(unplaced) {
		return "", unplaced[0]
	}
	if 0 < len(e.reviews) {
		return "", e.reviews[0].Reason
	}
	return out, ""
}

func (c *templateConverter) extractDefines(nodes []node, filename string) []node {
	body := make([]node, 0, len(nodes))
	for _, n := range nodes {
		def, isDefine := n.(*defineNode)
		if !isDefine {
			body = append(body, n)
			continue
		}
		c.convertDefine(def, filename)
	}
	if onlyWhitespace(body) {
		return nil
	}
	return body
}

func onlyWhitespace(nodes []node) bool {
	for _, n := range nodes {
		t, isText := n.(*textNode)
		if !isText {
			return false
		}
		if "" != strings.TrimSpace(t.raw) {
			return false
		}
	}
	return true
}

func (c *templateConverter) convertDefine(def *defineNode, filename string) {
	e := newEmitter(filename, c.result)
	e.emit(def.body, c.rootScope(filename))
	skeleton := e.buf.String()

	if !looksLikeMapping(skeleton) {
		if text, ok := c.textDefine(def, filename); ok {
			c.addPartial(def.name, scalarNode(text))
			return
		}
		if node, ok := c.textNodeFor(def.body, filename); ok {
			c.addPartial(def.name, node)
			return
		}
	}

	rewritten, unplaced, err := e.spans.apply(skeleton)
	if nil != err || 0 < len(unplaced) || 0 < len(e.reviews) || 0 < len(e.helmKept) {
		if text, ok := c.textDefine(def, filename); ok {
			c.addPartial(def.name, scalarNode(text))
			return
		}
		if node, ok := c.textNodeFor(def.body, filename); ok {
			c.addPartial(def.name, node)
			return
		}
		c.helmDefine(def, filename, "text body: "+c.lastTextError+" / "+defineFallbackReason(err, unplaced, e.reviews))
		return
	}
	trimmed := strings.TrimSpace(rewritten)
	if "" == trimmed {
		c.addPartial(def.name, scalarNode(strings.TrimSpace(skeleton)))
		return
	}
	var parsed yaml.Node
	if err := yaml.Unmarshal([]byte(rewritten), &parsed); nil != err || 0 == len(parsed.Content) {
		c.addPartial(def.name, scalarNode(trimmed))
		return
	}
	node := parsed.Content[0]
	if yaml.MappingNode != node.Kind {
		if text, ok := c.textDefine(def, filename); ok {
			c.addPartial(def.name, scalarNode(text))
			return
		}
		c.addPartial(def.name, scalarNode(trimmed))
		return
	}
	c.addPartial(def.name, node)
}

func (c *templateConverter) textDefine(def *defineNode, filename string) (string, bool) {
	e := newEmitter(filename, c.result)
	e.textOnly = true
	e.emit(def.body, c.rootScope(filename))
	text := strings.TrimSpace(e.buf.String())
	if "" == text || strings.Contains(text, "{{") || 0 < len(e.reviews) || 0 < len(e.helmKept) {
		return "", false
	}
	if hasEagerSideEffect(text) {
		return "", false
	}
	return text, true
}

func hasEagerSideEffect(expr string) bool {
	if !strings.Contains(expr, "ternary") {
		return false
	}
	return strings.Contains(expr, "fail") || strings.Contains(expr, "required")
}

func looksLikeMapping(skeleton string) bool {
	for _, line := range strings.Split(skeleton, "\n") {
		trimmed := strings.TrimSpace(line)
		if "" == trimmed || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
			continue
		}
		if strings.HasPrefix(trimmed, "$") {
			continue
		}
		if idx := strings.Index(trimmed, ": "); 0 < idx {
			return true
		}
		if strings.HasSuffix(trimmed, ":") {
			return true
		}
	}
	return false
}

func defineFallbackReason(err error, unplaced []string, reviews []ReviewItem) string {
	switch {
	case nil != err:
		return err.Error()
	case 0 < len(unplaced):
		return unplaced[0]
	case 0 < len(reviews):
		return reviews[0].Reason
	}
	return "cannot be expressed as keramos YAML"
}

func (c *templateConverter) helmDefine(def *defineNode, filename, reason string) {
	c.review(filename, def.line, "define "+def.name+": "+reason, "")
}

func (c *templateConverter) helpersDocument() ([]byte, error) {
	if 0 == len(c.order) {
		return nil, nil
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, name := range c.order {
		root.Content = append(root.Content, scalarNode(name), c.partials[name])
	}
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); nil != err {
		return nil, err
	}
	if err := enc.Close(); nil != err {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func (c *templateConverter) convertText(content, filename string) (string, bool) {
	nodes, err := buildNodes(content)
	if nil != err {
		c.review(filename, 1, err.Error(), "")
		return "", false
	}
	e := newEmitter(filename, c.result)
	e.textOnly = true
	e.emit(nodes, c.rootScope(filename))
	if 0 == len(e.reviews) && 0 == len(e.helmKept) {
		return e.buf.String(), false
	}

	node, ok := c.textNodeFor(nodes, filename)
	if !ok {
		e.flushReviews()
		for _, reason := range e.helmKept {
			c.review(filename, 1, reason, "")
		}
		return "", false
	}
	out, marshalErr := yaml.Marshal(map[string]any{"message": node})
	if nil != marshalErr {
		c.review(filename, 1, marshalErr.Error(), "")
		return "", false
	}
	return string(out), true
}

var reYAMLErrorLine = regexp.MustCompile(`line (\d+):`)

func yamlErrorContext(err error, text string) (int, string) {
	m := reYAMLErrorLine.FindStringSubmatch(err.Error())
	if nil == m {
		return 1, ""
	}
	num, convErr := strconv.Atoi(m[1])
	if nil != convErr {
		return 1, ""
	}
	lines := strings.Split(text, "\n")
	if num < 1 || num > len(lines) {
		return num, ""
	}
	return num, strings.TrimSpace(lines[num-1])
}
