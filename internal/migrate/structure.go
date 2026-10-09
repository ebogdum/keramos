package migrate

import (
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type lineRange struct {
	start int
	end   int
}

func (r lineRange) covers(line int) bool {
	return line >= r.start && line <= r.end
}

func (r lineRange) size() int {
	return r.end - r.start
}

type branchSpan struct {
	cond string
	span lineRange
}

type blockSpan struct {
	kind     string
	expr     string
	loopVar  string
	branches []branchSpan
	line     int
	column   int
}

type actionSpan struct {
	line    int
	column  int
	expr    string
	partial string
}

type structurer struct {
	blocks     []*blockSpan
	actions    []*actionSpan
	fragments  map[*blockSpan]map[int]*yaml.Node
	fragColumn map[*blockSpan]int
}

func (s *structurer) apply(text string) (string, []string, error) {
	text, docs, err := s.decodeWithFallback(text)
	if nil != err {
		return "", nil, err
	}
	unplaced := make([]string, 0, 4)

	if noStructure(docs) {
		if 0 < len(s.actions)+len(s.blocks) {
			unplaced = append(unplaced, unsupported(
				"template renders no YAML of its own; its %d template directives need converting by hand",
				len(s.actions)+len(s.blocks)).Error())
		}
		return "", unplaced, nil
	}

	for _, action := range s.actions {
		if !placeAction(docs, action) {
			unplaced = append(unplaced, unsupported(
				"expression on line %d has no YAML structure to attach to", action.line).Error())
		}
	}

	ordered := append([]*blockSpan{}, s.blocks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return blockExtent(ordered[i]).size() < blockExtent(ordered[j]).size()
	})
	for _, block := range ordered {
		if !s.placeBlockWithFragments(docs, block) {
			unplaced = append(unplaced, unsupported(
				"control flow on line %d has no YAML structure to attach to", block.line).Error())
		}
	}

	collisions := make([]string, 0, 4)
	for _, doc := range docs {
		if 0 < len(doc.Content) {
			coalesceMerges(doc.Content[0])
			collisions = append(collisions, dedupeKeys(doc.Content[0])...)
		}
	}
	if 0 < len(collisions) {
		unplaced = append(unplaced, unsupported(
			"branches define the same keys (%s); the first definition was kept",
			strings.Join(uniqueStrings(collisions), ", ")).Error())
	}

	rendered, err := encodeDocuments(docs)
	if nil != err {
		return "", unplaced, err
	}
	return rendered, unplaced, nil
}

const maxBranchCuts = 3

func (s *structurer) decodeWithFallback(text string) (string, []*yaml.Node, error) {
	original := text
	firstErr := error(nil)
	for attempt := 0; attempt < maxBranchCuts; attempt++ {
		docs, err := decodeDocuments(text)
		if nil == err {
			return text, docs, nil
		}
		if nil == firstErr {
			firstErr = err
		}
		line, ok := errorLine(err)
		if !ok {
			return original, nil, err
		}
		block, branch, found := s.branchAt(line)
		if !found {
			return original, nil, err
		}
		remaining, fragment := cutBranch(text, block.branches[branch].span)
		if "" == strings.TrimSpace(fragment) {
			return original, nil, err
		}
		s.rememberFragment(block, branch, fragment)
		text = remaining
	}
	docs, err := decodeDocuments(text)
	if nil != err {
		s.fragments = nil
		s.fragColumn = nil
		return original, nil, firstErr
	}
	return text, docs, nil
}

func errorLine(err error) (int, bool) {
	m := reYAMLErrorLine.FindStringSubmatch(err.Error())
	if nil == m {
		return 0, false
	}
	num, convErr := strconv.Atoi(m[1])
	return num, nil == convErr
}

func (s *structurer) branchAt(line int) (*blockSpan, int, bool) {
	var best *blockSpan
	bestBranch := 0
	for _, block := range s.blocks {
		for i, br := range block.branches {
			if !br.span.covers(line) {
				continue
			}
			if nil == best || br.span.size() < best.branches[bestBranch].span.size() {
				best = block
				bestBranch = i
			}
		}
	}
	if nil == best {
		return nil, 0, false
	}
	if _, taken := s.fragments[best][bestBranch]; taken {
		return nil, 0, false
	}
	return best, bestBranch, true
}

func cutBranch(text string, span lineRange) (string, string) {
	lines := strings.Split(text, "\n")
	kept := make([]string, len(lines))
	copy(kept, lines)
	cut := make([]string, 0, span.size()+1)
	for i := span.start - 1; i < span.end && i < len(lines); i++ {
		if i < 0 {
			continue
		}
		cut = append(cut, lines[i])
		kept[i] = ""
	}
	return strings.Join(kept, "\n"), strings.Join(cut, "\n")
}

func (s *structurer) rememberFragment(block *blockSpan, branch int, fragment string) {
	node, err := parseFragment(fragment)
	if nil != err {
		return
	}
	if nil == s.fragColumn {
		s.fragColumn = map[*blockSpan]int{}
	}
	if column := fragmentColumn(fragment); 0 < column {
		s.fragColumn[block] = column
	}
	if nil == s.fragments {
		s.fragments = map[*blockSpan]map[int]*yaml.Node{}
	}
	if nil == s.fragments[block] {
		s.fragments[block] = map[int]*yaml.Node{}
	}
	s.fragments[block][branch] = node
}

func fragmentColumn(fragment string) int {
	best := -1
	for _, line := range strings.Split(fragment, "\n") {
		if "" == strings.TrimSpace(line) {
			continue
		}
		lead := len(line) - len(strings.TrimLeft(line, " \t"))
		if 0 > best || lead < best {
			best = lead
		}
	}
	if 0 > best {
		return 0
	}
	return best + 1
}

func parseFragment(fragment string) (*yaml.Node, error) {
	trimmed := dedentFragment(fragment)
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(trimmed), &doc); nil != err {
		return nil, err
	}
	if 0 == len(doc.Content) {
		return nil, unsupported("empty fragment")
	}
	return doc.Content[0], nil
}

func dedentFragment(fragment string) string {
	lines := strings.Split(fragment, "\n")
	indent := -1
	for _, line := range lines {
		if "" == strings.TrimSpace(line) {
			continue
		}
		lead := len(line) - len(strings.TrimLeft(line, " \t"))
		if 0 > indent || lead < indent {
			indent = lead
		}
	}
	if 0 >= indent {
		return fragment
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) >= indent {
			out = append(out, line[indent:])
			continue
		}
		out = append(out, strings.TrimLeft(line, " \t"))
	}
	return strings.Join(out, "\n")
}

func noStructure(docs []*yaml.Node) bool {
	for _, doc := range docs {
		if 0 < len(doc.Content) && !isEmptyNode(doc.Content[0]) {
			return false
		}
	}
	return true
}

func blockExtent(b *blockSpan) lineRange {
	out := b.branches[0].span
	for _, br := range b.branches {
		if br.span.start < out.start {
			out.start = br.span.start
		}
		if br.span.end > out.end {
			out.end = br.span.end
		}
	}
	return out
}

func decodeDocuments(text string) ([]*yaml.Node, error) {
	decoder := yaml.NewDecoder(strings.NewReader(text))
	docs := make([]*yaml.Node, 0, 4)
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if nil != err {
			if "EOF" == err.Error() {
				return docs, nil
			}
			return nil, err
		}
		docs = append(docs, &doc)
	}
}

func encodeDocuments(docs []*yaml.Node) (string, error) {
	parts := make([]string, 0, len(docs))
	for _, doc := range docs {
		if 0 == len(doc.Content) || isEmptyNode(doc.Content[0]) {
			continue
		}
		var buf strings.Builder
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(doc.Content[0]); nil != err {
			return "", err
		}
		if err := enc.Close(); nil != err {
			return "", err
		}
		parts = append(parts, buf.String())
	}
	return strings.Join(parts, "---\n"), nil
}

func isEmptyNode(n *yaml.Node) bool {
	if nil == n {
		return true
	}
	switch n.Kind {
	case yaml.MappingNode, yaml.SequenceNode:
		return 0 == len(n.Content)
	case yaml.ScalarNode:
		return "" == strings.TrimSpace(n.Value) || "null" == n.Value
	}
	return false
}

func nodeExtent(n *yaml.Node) int {
	last := n.Line
	for _, child := range n.Content {
		if reach := nodeExtent(child); reach > last {
			last = reach
		}
	}
	return last
}

func placeAction(docs []*yaml.Node, action *actionSpan) bool {
	entry := mergePair(actionNodeValue(action), action.line)
	return insertAt(docs, lineRange{start: action.line, end: action.line}, action.column, entry)
}

func (s *structurer) fragmentNodes(block *blockSpan, branch int) ([]*yaml.Node, bool) {
	node, ok := s.fragments[block][branch]
	if !ok || nil == node {
		return nil, false
	}
	return node.Content, true
}

func placeBlock(docs []*yaml.Node, block *blockSpan) bool {
	extent := blockExtent(block)
	for _, doc := range docs {
		if 0 == len(doc.Content) {
			continue
		}
		root := doc.Content[0]
		if covered(root, extent) {
			sections := make([]*yaml.Node, len(block.branches))
			for i := range sections {
				sections[i] = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			idx, ok := branchFor(block, root.Line)
			if !ok {
				idx = 0
			}
			sections[idx] = root
			doc.Content[0] = buildBlockFromSections(block, sections, false)
			return true
		}
		if wrapInNode(root, block, extent) {
			return true
		}
	}
	entry := mergePair(buildBlockNode(block, emptySections(block), false), extent.start)
	return insertAt(docs, extent, block.column, entry)
}

func (s *structurer) placeBlockWithFragments(docs []*yaml.Node, block *blockSpan) bool {
	if 0 == len(s.fragments[block]) {
		return placeBlock(docs, block)
	}
	branchNodes := emptySections(block)
	sequence := false
	for i := range block.branches {
		nodes, ok := s.fragmentNodes(block, i)
		if !ok {
			continue
		}
		branchNodes[i] = nodes
		if node := s.fragments[block][i]; yaml.SequenceNode == node.Kind {
			sequence = true
		}
	}
	directive := buildBlockNode(block, branchNodes, sequence)
	entry := mergePair(directive, blockExtent(block).start)
	column := block.column
	if fragCol, ok := s.fragColumn[block]; ok {
		column = fragCol
	}
	return insertAt(docs, blockExtent(block), column, entry)
}

func emptySections(block *blockSpan) [][]*yaml.Node {
	out := make([][]*yaml.Node, len(block.branches))
	return out
}

func covered(n *yaml.Node, extent lineRange) bool {
	return n.Line >= extent.start && nodeExtent(n) <= extent.end
}

func wrapInNode(n *yaml.Node, block *blockSpan, extent lineRange) bool {
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if wrapInNode(n.Content[i+1], block, extent) {
				return true
			}
		}
		return wrapMappingChildren(n, block, extent)
	case yaml.SequenceNode:
		for _, item := range n.Content {
			if wrapInNode(item, block, extent) {
				return true
			}
		}
		return wrapSequenceChildren(n, block, extent)
	}
	return false
}

func wrapMappingChildren(n *yaml.Node, block *blockSpan, extent lineRange) bool {
	branchNodes := make([][]*yaml.Node, len(block.branches))
	kept := make([]*yaml.Node, 0, len(n.Content))
	hit := false
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		idx, inRange := branchFor(block, key.Line)
		if !inRange {
			kept = append(kept, key, value)
			continue
		}
		hit = true
		branchNodes[idx] = append(branchNodes[idx], key, value)
	}
	if !hit {
		return false
	}
	n.Content = append(kept, mergePair(buildBlockNode(block, branchNodes, false), extent.start)...)
	return true
}

func wrapSequenceChildren(n *yaml.Node, block *blockSpan, extent lineRange) bool {
	branchNodes := make([][]*yaml.Node, len(block.branches))
	kept := make([]*yaml.Node, 0, len(n.Content))
	slot := -1
	for _, item := range n.Content {
		idx, inRange := branchFor(block, item.Line)
		if !inRange {
			kept = append(kept, item)
			continue
		}
		if 0 > slot {
			slot = len(kept)
			kept = append(kept, nil)
		}
		branchNodes[idx] = append(branchNodes[idx], item)
	}
	if 0 > slot {
		return false
	}
	directive := mappingNode("$merge", buildBlockNode(block, branchNodes, true))
	directive.Line = extent.start
	kept[slot] = directive
	n.Content = kept
	return true
}

func branchFor(block *blockSpan, line int) (int, bool) {
	for i, br := range block.branches {
		if br.span.covers(line) {
			return i, true
		}
	}
	return 0, false
}

func insertAt(docs []*yaml.Node, span lineRange, column int, entry []*yaml.Node) bool {
	for _, doc := range docs {
		if 0 == len(doc.Content) {
			continue
		}
		if insertInNode(doc.Content[0], span, column, entry) {
			return true
		}
	}
	return false
}

func insertInNode(n *yaml.Node, span lineRange, column int, entry []*yaml.Node) bool {
	switch n.Kind {
	case yaml.MappingNode:
		return insertInMapping(n, span, column, entry)
	case yaml.SequenceNode:
		return insertInSequence(n, span, column, entry)
	}
	return false
}

func childColumn(n *yaml.Node) int {
	if 0 == len(n.Content) {
		return n.Column
	}
	return n.Content[0].Column
}

func insertInMapping(n *yaml.Node, span lineRange, column int, entry []*yaml.Node) bool {
	own := childColumn(n)
	if column > own {
		if descendMapping(n, span, column, entry) {
			return true
		}
	}
	if n.Line > span.start {
		return false
	}
	n.Content = insertPairInOrder(n.Content, entry, span.start)
	return true
}

func insertPairInOrder(content []*yaml.Node, entry []*yaml.Node, line int) []*yaml.Node {
	out := make([]*yaml.Node, 0, len(content)+2)
	inserted := false
	for i := 0; i+1 < len(content); i += 2 {
		if !inserted && content[i].Line > line {
			out = append(out, entry...)
			inserted = true
		}
		out = append(out, content[i], content[i+1])
	}
	if !inserted {
		out = append(out, entry...)
	}
	return out
}

func descendMapping(n *yaml.Node, span lineRange, column int, entry []*yaml.Node) bool {
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if key.Line >= span.start {
			continue
		}
		limit := nextKeyLine(n, i)
		if 0 < limit && limit <= span.start {
			continue
		}
		if isEmptyNode(value) {
			holder := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Line: span.start, Column: column}
			holder.Content = append(holder.Content, entry...)
			n.Content[i+1] = holder
			return true
		}
		return insertInNode(value, span, column, entry)
	}
	return false
}

func nextKeyLine(n *yaml.Node, i int) int {
	if i+2 < len(n.Content) {
		return n.Content[i+2].Line
	}
	return 0
}

func insertInSequence(n *yaml.Node, span lineRange, column int, entry []*yaml.Node) bool {
	own := childColumn(n)
	if column > own {
		for _, item := range n.Content {
			if item.Line < span.start && nodeExtent(item) >= span.start {
				if insertInNode(item, span, column, entry) {
					return true
				}
			}
		}
		if last := lastItemBefore(n, span.start); nil != last && column >= childColumn(last) {
			if insertInNode(last, span, column, entry) {
				return true
			}
		}
	}
	if n.Line > span.start {
		return false
	}
	wrapper := mappingNode("$merge", entry[1])
	wrapper.Line = span.start
	out := make([]*yaml.Node, 0, len(n.Content)+1)
	inserted := false
	for _, item := range n.Content {
		if !inserted && item.Line > span.start {
			out = append(out, wrapper)
			inserted = true
		}
		out = append(out, item)
	}
	if !inserted {
		out = append(out, wrapper)
	}
	n.Content = out
	return true
}

func lastItemBefore(n *yaml.Node, line int) *yaml.Node {
	var best *yaml.Node
	for _, item := range n.Content {
		if item.Line < line {
			best = item
		}
	}
	return best
}

func mergePair(value *yaml.Node, line int) []*yaml.Node {
	key := scalarNode("$merge")
	key.Line = line
	value.Line = line
	return []*yaml.Node{key, value}
}

func buildBlockNode(block *blockSpan, branchNodes [][]*yaml.Node, sequence bool) *yaml.Node {
	for len(branchNodes) < len(block.branches) {
		branchNodes = append(branchNodes, nil)
	}
	sections := make([]*yaml.Node, 0, len(branchNodes))
	for _, nodes := range branchNodes {
		sections = append(sections, sectionNode(nodes, sequence))
	}
	return buildBlockFromSections(block, sections, sequence)
}

func buildBlockFromSections(block *blockSpan, sections []*yaml.Node, sequence bool) *yaml.Node {
	if "each" == block.kind {
		if 1 > len(sections) {
			return sectionNode(nil, sequence)
		}
		each := mappingNode(
			"$each", scalarNode("${"+block.expr+"}"),
			"$as", scalarNode(block.loopVar),
			"$yield", sections[0],
		)
		if 1 == len(sections) || isEmptyNode(sections[1]) {
			return each
		}
		return mappingNode("$if", scalarNode("${"+block.expr+"}"), "$then", each, "$else", sections[1])
	}

	var out *yaml.Node
	last := len(block.branches) - 1
	if "" == block.branches[last].cond {
		out = sections[last]
		last--
	}
	for i := last; i >= 0; i-- {
		branch := mappingNode("$if", scalarNode("${"+block.branches[i].cond+"}"), "$then", sections[i])
		if nil != out && !isEmptyNode(out) {
			branch.Content = append(branch.Content, scalarNode("$else"), out)
		}
		out = branch
	}
	if nil == out {
		return sectionNode(nil, sequence)
	}
	return out
}

func sectionNode(nodes []*yaml.Node, sequence bool) *yaml.Node {
	if sequence {
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: nodes}
	}
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: nodes}
	coalesceMerges(node)
	return node
}

func plainIncludeExpr(action *actionSpan) bool {
	return action.expr == "include "+quoteLiteral(action.partial)+" | fromYaml"
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func dedupeKeys(n *yaml.Node) []string {
	if nil == n {
		return nil
	}
	dropped := make([]string, 0, 2)
	for _, child := range n.Content {
		dropped = append(dropped, dedupeKeys(child)...)
	}
	if yaml.MappingNode != n.Kind {
		return dropped
	}
	seen := make(map[string]bool, len(n.Content)/2)
	kept := make([]*yaml.Node, 0, len(n.Content))
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if seen[key.Value] {
			dropped = append(dropped, key.Value)
			continue
		}
		seen[key.Value] = true
		kept = append(kept, key, value)
	}
	n.Content = kept
	return dropped
}

func coalesceMerges(n *yaml.Node) {
	if nil == n {
		return
	}
	for _, child := range n.Content {
		coalesceMerges(child)
	}
	if yaml.MappingNode != n.Kind {
		return
	}
	merged := make([]*yaml.Node, 0, 2)
	kept := make([]*yaml.Node, 0, len(n.Content))
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]
		if "$merge" != key.Value {
			kept = append(kept, key, value)
			continue
		}
		if yaml.SequenceNode == value.Kind {
			merged = append(merged, value.Content...)
			continue
		}
		merged = append(merged, value)
	}
	if 0 == len(merged) {
		return
	}
	if 1 == len(merged) {
		n.Content = append(kept, scalarNode("$merge"), merged[0])
		return
	}
	n.Content = append(kept, scalarNode("$merge"), sequenceNode(merged))
}

func actionNodeValue(action *actionSpan) *yaml.Node {
	if "" != action.partial && plainIncludeExpr(action) {
		return mappingNode("$include", scalarNode(action.partial))
	}
	return scalarNode("${" + action.expr + "}")
}

func mappingNode(pairs ...any) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for i := 0; i+1 < len(pairs); i += 2 {
		key, _ := pairs[i].(string)
		value, _ := pairs[i+1].(*yaml.Node)
		if nil == value {
			continue
		}
		node.Content = append(node.Content, scalarNode(key), value)
	}
	return node
}

func sequenceNode(items []*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: items}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}
