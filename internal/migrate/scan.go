package migrate

import (
	"strings"
)

type token struct {
	text     string
	action   bool
	body     string
	trimLeft bool
	trimRigh bool
	ownLine  bool
	indent   string
	line     int
	start    int
	end      int
}

func scanTemplate(src string) []token {
	tokens := make([]token, 0, 64)
	i := 0
	line := 1
	for i < len(src) {
		start := strings.Index(src[i:], "{{")
		if 0 > start {
			tokens = append(tokens, token{text: src[i:], line: line, start: i, end: len(src)})
			break
		}
		start += i
		if start > i {
			chunk := src[i:start]
			tokens = append(tokens, token{text: chunk, line: line, start: i, end: start})
			line += strings.Count(chunk, "\n")
		}
		end := findActionEnd(src, start)
		if 0 > end {
			tokens = append(tokens, token{text: src[start:], line: line, start: start, end: len(src)})
			break
		}
		raw := src[start:end]
		body := strings.TrimSuffix(strings.TrimPrefix(raw, "{{"), "}}")
		trimLeft := strings.HasPrefix(body, "-") && (1 == len(body) || isSpace(body[1]))
		if trimLeft {
			body = body[1:]
		}
		trimRight := strings.HasSuffix(body, "-") && (1 == len(body) || isSpace(body[len(body)-2]))
		if trimRight {
			body = body[:len(body)-1]
		}
		own, indent := ownLineContext(src, start, end)
		tokens = append(tokens, token{
			action:   true,
			body:     strings.TrimSpace(body),
			trimLeft: trimLeft,
			trimRigh: trimRight,
			ownLine:  own,
			indent:   indent,
			line:     line,
			start:    start,
			end:      end,
		})
		line += strings.Count(raw, "\n")
		i = end
	}
	return tokens
}

func isSpace(c byte) bool {
	return ' ' == c || '\t' == c || '\n' == c || '\r' == c
}

func findActionEnd(src string, start int) int {
	if isCommentAction(src, start) {
		return commentActionEnd(src, start)
	}
	i := start + 2
	var quote byte
	for i < len(src) {
		c := src[i]
		switch {
		case 0 != quote:
			if '\\' == c && i+1 < len(src) {
				i++
			} else if c == quote {
				quote = 0
			}
		case '"' == c || '\'' == c || '`' == c:
			quote = c
		case '}' == c && i+1 < len(src) && '}' == src[i+1]:
			return i + 2
		}
		i++
	}
	return -1
}

func isCommentAction(src string, start int) bool {
	rest := src[start:]
	return strings.HasPrefix(rest, "{{/*") || strings.HasPrefix(rest, "{{- /*") || strings.HasPrefix(rest, "{{ /*")
}

func commentActionEnd(src string, start int) int {
	idx := strings.Index(src[start:], "*/")
	if 0 > idx {
		return -1
	}
	i := start + idx + 2
	for i < len(src) && (' ' == src[i] || '\t' == src[i] || '-' == src[i]) {
		i++
	}
	if i+1 < len(src) && '}' == src[i] && '}' == src[i+1] {
		return i + 2
	}
	return -1
}

func ownLineContext(src string, start, end int) (bool, string) {
	lineStart := strings.LastIndexByte(src[:start], '\n') + 1
	prefix := src[lineStart:start]
	if "" != strings.TrimSpace(prefix) {
		return false, ""
	}
	rest := src[end:]
	if idx := strings.IndexByte(rest, '\n'); 0 <= idx {
		rest = rest[:idx]
	}
	if "" != strings.TrimSpace(rest) {
		return false, ""
	}
	return true, prefix
}

type node interface{}

type textNode struct {
	raw            string
	dropLeadLine   bool
	dropTrailInden bool
}

type actionNode struct {
	body    string
	ownLine bool
	indent  string
	line    int
	trimL   bool
	trimR   bool
	raw     string
}

type branch struct {
	cond      string
	body      []node
	line      int
	trimStart bool
	trimEnd   bool
}

type blockNode struct {
	kind      string
	branches  []branch
	ownLine   bool
	indent    string
	line      int
	endOwn    bool
	openTrimL bool
	openTrimR bool
	endTrimL  bool
	endTrimR  bool
	raw       string
}

type defineNode struct {
	name string
	body []node
	line int
	raw  string
}

type parser struct {
	tokens []token
	pos    int
	src    string
}

func buildNodes(src string) ([]node, error) {
	p := &parser{tokens: scanTemplate(src), src: src}
	nodes, terminator, err := p.parseList(nil)
	if nil != err {
		return nil, err
	}
	for "" != terminator {
		p.pos++
		more, next, moreErr := p.parseList(nil)
		if nil != moreErr {
			return nil, moreErr
		}
		nodes = append(nodes, more...)
		terminator = next
		if p.pos >= len(p.tokens) {
			break
		}
	}
	return nodes, nil
}

var blockStarters = map[string]bool{"if": true, "range": true, "with": true}

var templateKeywords = []string{"else if", "if", "else", "range", "with", "end", "define", "block", "template"}

func directiveWord(body string) string {
	trimmed := strings.TrimSpace(body)
	for _, keyword := range templateKeywords {
		if !strings.HasPrefix(trimmed, keyword) {
			continue
		}
		rest := trimmed[len(keyword):]
		if "" == rest || startsArgument(rest) {
			if "else if" == keyword {
				return "else"
			}
			return keyword
		}
	}
	fields := strings.Fields(trimmed)
	if 0 == len(fields) {
		return ""
	}
	return fields[0]
}

func startsArgument(rest string) bool {
	switch rest[0] {
	case ' ', '\t', '.', '(', '$', '"', '\'':
		return true
	}
	return false
}

func directiveArgument(body, keyword string) string {
	trimmed := strings.TrimSpace(body)
	return strings.TrimSpace(strings.TrimPrefix(trimmed, keyword))
}

func (p *parser) parseList(stopAt map[string]bool) ([]node, string, error) {
	nodes := make([]node, 0, 16)
	for p.pos < len(p.tokens) {
		tok := p.tokens[p.pos]
		if !tok.action {
			nodes = append(nodes, &textNode{raw: tok.text})
			p.pos++
			continue
		}
		word := directiveWord(tok.body)
		if nil != stopAt && stopAt[word] {
			return nodes, word, nil
		}
		switch {
		case blockStarters[word]:
			blk, err := p.parseBlock(tok)
			if nil != err {
				return nil, "", err
			}
			nodes = append(nodes, blk)
		case "define" == word || "block" == word:
			def, err := p.parseDefine(tok)
			if nil != err {
				return nil, "", err
			}
			nodes = append(nodes, def)
		case "end" == word || "else" == word:
			return nodes, word, nil
		default:
			nodes = append(nodes, &actionNode{
				body: tok.body, ownLine: tok.ownLine, indent: tok.indent,
				line: tok.line, trimL: tok.trimLeft, trimR: tok.trimRigh,
				raw: p.src[tok.start:tok.end],
			})
			p.pos++
		}
	}
	return nodes, "", nil
}

var blockStop = map[string]bool{"end": true, "else": true}

func (p *parser) parseBlock(start token) (*blockNode, error) {
	p.pos++
	blk := &blockNode{
		kind:      directiveWord(start.body),
		ownLine:   start.ownLine,
		indent:    start.indent,
		line:      start.line,
		openTrimL: start.trimLeft,
		openTrimR: start.trimRigh,
	}
	cond := directiveArgument(start.body, blk.kind)
	trimStart := start.trimRigh
	for {
		body, terminator, err := p.parseList(blockStop)
		if nil != err {
			return nil, err
		}
		if p.pos >= len(p.tokens) {
			return nil, unsupported("unterminated {{ %s }} block opened on line %d", blk.kind, blk.line)
		}
		tok := p.tokens[p.pos]
		p.pos++
		blk.branches = append(blk.branches, branch{
			cond: cond, body: body, line: start.line,
			trimStart: trimStart, trimEnd: tok.trimLeft,
		})
		trimStart = tok.trimRigh
		if "end" == terminator {
			blk.endOwn = tok.ownLine
			blk.endTrimL = tok.trimLeft
			blk.endTrimR = tok.trimRigh
			blk.raw = p.src[start.start:tok.end]
			return blk, nil
		}
		rest := directiveArgument(tok.body, "else")
		if strings.HasPrefix(rest, "if") {
			cond = directiveArgument(rest, "if")
			continue
		}
		cond = ""
	}
}

func (p *parser) parseDefine(start token) (*defineNode, error) {
	p.pos++
	name := directiveArgument(start.body, directiveWord(start.body))
	name = strings.Trim(name, `"'`)
	body, terminator, err := p.parseList(blockStop)
	if nil != err {
		return nil, err
	}
	if "end" != terminator {
		return nil, unsupported("unterminated {{ define %q }} opened on line %d", name, start.line)
	}
	end := p.tokens[p.pos]
	p.pos++
	return &defineNode{name: name, body: body, line: start.line, raw: p.src[start.start:end.end]}, nil
}
