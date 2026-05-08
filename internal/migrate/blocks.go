package migrate

import (
	"regexp"
	"strings"
)

// directiveKind classifies a Go template directive line.
type directiveKind int

const (
	dirNone directiveKind = iota
	dirIf
	dirElse
	dirElseIf
	dirEnd
	dirRange
	dirWith
)

// block represents a matched pair of template directives (if/end, with/end, range/end).
type block struct {
	kind      directiveKind
	startLine int
	endLine   int
	elseLine  int // -1 if no else
	condition string
	elseIfs   []elseIfBranch
}

type elseIfBranch struct {
	line      int
	condition string
}

// Regex patterns for block detection
var (
	reBlockIf      = regexp.MustCompile(`^\s*\{\{-?\s*if\s+(.+?)\s*-?\}\}\s*$`)
	reBlockElseIf  = regexp.MustCompile(`^\s*\{\{-?\s*else\s+if\s+(.+?)\s*-?\}\}\s*$`)
	reBlockElse    = regexp.MustCompile(`^\s*\{\{-?\s*else\s*-?\}\}\s*$`)
	reBlockEnd     = regexp.MustCompile(`^\s*\{\{-?\s*end\s*-?\}\}\s*$`)
	reBlockRange   = regexp.MustCompile(`^\s*\{\{-?\s*range\s+(.+?)\s*-?\}\}\s*$`)
	reBlockWith    = regexp.MustCompile(`^\s*\{\{-?\s*with\s+(.+?)\s*-?\}\}\s*$`)
	reToYamlLine   = regexp.MustCompile(`^\s*\{\{-?\s*toYaml\s+(\S+)\s*\|\s*nindent\s+\d+\s*-?\}\}\s*$`)

	// Standalone toYaml on its own line: {{- toYaml .Values.x | nindent N }}
	reToYamlStandalone = regexp.MustCompile(`^\s*\{\{-?\s*toYaml\s+\.Values\.(\S+)\s*\|\s*nindent\s+\d+\s*-?\}\}\s*$`)

	// Standalone include on its own line (already handled but repeated for block context)
	reIncludeStandalone = regexp.MustCompile(`^\s*\{\{-?\s*include\s+"([^"]+)"\s+\.\s*\|\s*nindent\s+\d+\s*-?\}\}\s*$`)

	// Patterns requiring manual review even inside blocks. Note: keramos's
	// runtime *does* implement tpl/lookup/dict/printf/index/get; the
	// migrator only flags forms whose mechanical translation isn't safe.
	reComplexAssign = regexp.MustCompile(`:=`)
	reComplexDollar = regexp.MustCompile(`\$[a-zA-Z]`)

	// Simple-form patterns that the migrator CAN translate. Entire-line
	// `{{ ... }}` constructs only — anything mid-string falls through to
	// the manual-review path.
	reSimpleTpl    = regexp.MustCompile(`^(\s*)\{\{-?\s*tpl\s+\.Values\.([^\s}]+)\s+\.\s*-?\}\}\s*$`)
	reSimpleLookup = regexp.MustCompile(`^(\s*)\{\{-?\s*lookup\s+("[^"]*")\s+("[^"]*")\s+("[^"]*")\s+("[^"]*")\s*-?\}\}\s*$`)
	reSimpleDict   = regexp.MustCompile(`^(\s*)\{\{-?\s*dict\s+(.+?)\s*-?\}\}\s*$`)
	reSimplePrintf = regexp.MustCompile(`\{\{-?\s*printf\s+("[^"]*")\s+(.+?)\s*-?\}\}`)
	reSimpleIndex  = regexp.MustCompile(`\{\{-?\s*index\s+\.Values\.([^\s}]+)\s+("[^"]+")\s*-?\}\}`)

	// $.X root references inside `range`/`with` bodies — translate to the
	// keramos root namespace `values.X`.
	reDotRoot = regexp.MustCompile(`\$\.([A-Za-z_][A-Za-z0-9_.]*)`)
	// `range $i, $v := .Values.x` — multi-var range over slice/map.
	reRangeIndexValue = regexp.MustCompile(`^(\s*)\{\{-?\s*range\s+\$([A-Za-z_][A-Za-z0-9_]*)\s*,\s*\$([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*\.Values\.([^\s}]+)\s*-?\}\}\s*$`)
	// `range $v := .Values.x` — single-var range.
	reRangeValueOnly = regexp.MustCompile(`^(\s*)\{\{-?\s*range\s+\$([A-Za-z_][A-Za-z0-9_]*)\s*:=\s*\.Values\.([^\s}]+)\s*-?\}\}\s*$`)
)

// rewriteSimpleHelmFunc walks a single line and rewrites the simple-form
// occurrences of tpl/lookup/dict/printf/index/$.X/range-with-vars to keramos's
// `${...}` syntax. Returns the rewritten line. Lines not matching any
// pattern are returned unchanged.
func rewriteSimpleHelmFunc(line string) string {
	// Multi-var range `{{ range $i, $v := .Values.x }}` → keramos's $each with $as.
	if m := reRangeIndexValue.FindStringSubmatch(line); nil != m {
		return m[1] + "$each: ${values." + m[4] + "}\n" + m[1] + "$as: " + m[3]
	}
	if m := reRangeValueOnly.FindStringSubmatch(line); nil != m {
		return m[1] + "$each: ${values." + m[3] + "}\n" + m[1] + "$as: " + m[2]
	}

	// $.X root references → values.X.
	if reDotRoot.MatchString(line) {
		line = reDotRoot.ReplaceAllString(line, `${values.$1}`)
	}

	if m := reSimpleTpl.FindStringSubmatch(line); nil != m {
		return m[1] + "${tpl Values." + m[2] + "}"
	}
	if m := reSimpleLookup.FindStringSubmatch(line); nil != m {
		return m[1] + "${lookup " + m[2] + " " + m[3] + " " + m[4] + " " + m[5] + "}"
	}
	if m := reSimpleDict.FindStringSubmatch(line); nil != m {
		return m[1] + "${dict " + m[2] + "}"
	}
	line = reSimplePrintf.ReplaceAllString(line, "${printf $1 $2}")
	line = reSimpleIndex.ReplaceAllString(line, "${get Values.$1 $2}")
	return line
}

// classifyLine returns the directive kind and extracted condition/argument.
func classifyLine(line string) (directiveKind, string) {
	if m := reBlockElseIf.FindStringSubmatch(line); nil != m {
		return dirElseIf, m[1]
	}
	if reBlockElse.MatchString(line) {
		return dirElse, ""
	}
	if reBlockEnd.MatchString(line) {
		return dirEnd, ""
	}
	if m := reBlockIf.FindStringSubmatch(line); nil != m {
		return dirIf, m[1]
	}
	if m := reBlockRange.FindStringSubmatch(line); nil != m {
		return dirRange, m[1]
	}
	if m := reBlockWith.FindStringSubmatch(line); nil != m {
		return dirWith, m[1]
	}
	return dirNone, ""
}

// matchBlocks pairs opening/closing directives with nesting awareness.
// Returns a map from startLine -> block.
func matchBlocks(lines []string) []block {
	type stackEntry struct {
		kind      directiveKind
		line      int
		condition string
		elseLine  int
		elseIfs   []elseIfBranch
	}

	stack := make([]stackEntry, 0, 8)
	blocks := make([]block, 0, 8)
	lineCount := len(lines)

	for i := range lineCount {
		kind, cond := classifyLine(lines[i])

		switch kind {
		case dirIf, dirRange, dirWith:
			stack = append(stack, stackEntry{
				kind:      kind,
				line:      i,
				condition: cond,
				elseLine:  -1,
				elseIfs:   nil,
			})
		case dirElse:
			if len(stack) > 0 {
				stack[len(stack)-1].elseLine = i
			}
		case dirElseIf:
			if len(stack) > 0 {
				top := &stack[len(stack)-1]
				top.elseIfs = append(top.elseIfs, elseIfBranch{line: i, condition: cond})
			}
		case dirEnd:
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				blocks = append(blocks, block{
					kind:      top.kind,
					startLine: top.line,
					endLine:   i,
					elseLine:  top.elseLine,
					condition: top.condition,
					elseIfs:   top.elseIfs,
				})
			}
		}
	}
	return blocks
}

// isComplexExpression checks if a condition or body line contains patterns
// that require manual review. The migrator stays conservative inside control
// flow bodies even though keramos's runtime supports tpl/lookup/dict/printf/index;
// the syntactic translation isn't always safe in nested contexts.
func isComplexExpression(s string) bool {
	if reComplexAssign.MatchString(s) {
		return true
	}
	if strings.Contains(s, "tpl ") {
		return true
	}
	if strings.Contains(s, "lookup ") {
		return true
	}
	if strings.Contains(s, "dict ") {
		return true
	}
	if strings.Contains(s, "printf ") {
		return true
	}
	if strings.Contains(s, "index ") {
		return true
	}
	return false
}

// hasComplexContent checks if any line in a range contains complex patterns.
func hasComplexContent(lines []string, from, to int) bool {
	for i := from; i <= to && i < len(lines); i++ {
		if isComplexExpression(lines[i]) {
			return true
		}
	}
	return false
}

// hasDollarVarRefs checks if any line in a range has $variable references
// (not ${ which is our keramos syntax).
func hasDollarVarRefs(lines []string, from, to int) bool {
	for i := from; i <= to && i < len(lines); i++ {
		if reComplexDollar.MatchString(lines[i]) {
			return true
		}
	}
	return false
}

// convertCondition converts a Helm template condition to a keramos expression.
func convertCondition(cond string) string {
	cond = strings.TrimSpace(cond)

	// Handle negation: not .Values.x -> !values.x
	if neg, ok := strings.CutPrefix(cond, "not "); ok {
		inner := convertConditionRef(strings.TrimSpace(neg))
		if "" == inner {
			return ""
		}
		return "!" + inner
	}

	// Handle comparison operators: eq, ne, gt, lt, ge, le
	for _, op := range []struct{ helm, keramos string }{
		{"eq ", "=="}, {"ne ", "!="}, {"gt ", ">"}, {"lt ", "<"}, {"ge ", ">="}, {"le ", "<="},
	} {
		if after, ok := strings.CutPrefix(cond, op.helm); ok {
			parts := strings.Fields(strings.TrimSpace(after))
			if 2 == len(parts) {
				left := convertConditionRef(parts[0])
				right := convertConditionRef(parts[1])
				if "" != left && "" != right {
					return left + " " + op.keramos + " " + right
				}
			}
			return ""
		}
	}

	// Handle "and" / "or" combinations
	if strings.HasPrefix(cond, "and ") || strings.HasPrefix(cond, "or ") {
		return ""
	}

	// Simple truthy check: .Values.x
	ref := convertConditionRef(cond)
	if "" != ref {
		return ref
	}

	return ""
}

// convertConditionRef converts a single reference inside a condition.
func convertConditionRef(ref string) string {
	ref = strings.TrimSpace(ref)

	if strings.HasPrefix(ref, ".Values.") {
		return "values." + ref[len(".Values."):]
	}

	replacements := map[string]string{
		".Release.Name":      "release.name",
		".Release.Namespace": "release.namespace",
		".Chart.Name":        "package.name",
		".Chart.Version":     "package.version",
		".Chart.AppVersion":  "package.appVersion",
	}
	if mapped, ok := replacements[ref]; ok {
		return mapped
	}

	// Quoted strings and numbers pass through
	if (strings.HasPrefix(ref, `"`) && strings.HasSuffix(ref, `"`)) ||
		(strings.HasPrefix(ref, `'`) && strings.HasSuffix(ref, `'`)) {
		return ref
	}
	if isNumeric(ref) {
		return ref
	}

	return ""
}

func isNumeric(s string) bool {
	if 0 == len(s) {
		return false
	}
	for _, ch := range s {
		if (ch < '0' || ch > '9') && '.' != ch && '-' != ch {
			return false
		}
	}
	return true
}

// isWithToYamlBody checks if the body of a with block is just {{- toYaml . | nindent N }}.
func isWithToYamlBody(lines []string, bodyStart, bodyEnd int) bool {
	nonEmpty := 0
	for i := bodyStart; i <= bodyEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if "" == trimmed {
			continue
		}
		nonEmpty++
		if !reToYamlLine.MatchString(lines[i]) {
			return false
		}
	}
	return 1 == nonEmpty
}

// isWithScalarBody checks if the body of a with block is a single line
// like `schedulerName: {{ . }}` or `priorityClassName: {{ . | quote }}`.
var reWithDotRef = regexp.MustCompile(`\{\{-?\s*\.\s*(\|\s*\w+\s*)?-?\}\}`)

func isWithScalarBody(lines []string, bodyStart, bodyEnd int) bool {
	nonEmpty := 0
	for i := bodyStart; i <= bodyEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if "" == trimmed {
			continue
		}
		nonEmpty++
	}
	return 1 == nonEmpty
}

// bodyHasNestedBlocks checks whether the body between from and to
// contains any nested block directives.
func bodyHasNestedBlocks(lines []string, from, to int) bool {
	for i := from; i <= to; i++ {
		kind, _ := classifyLine(lines[i])
		if dirNone != kind {
			return true
		}
	}
	return false
}

// convertTemplateContentV2 is the block-aware replacement for convertTemplateContent.
func convertTemplateContentV2(content, filename string, result *MigrateResult) string {
	lines := strings.Split(content, "\n")
	blocks := matchBlocks(lines)

	// Build a set of lines that are part of converted blocks so we skip them.
	converted := make(map[int]bool, len(lines))
	// Output replacement lines keyed by the start line of a block.
	replacements := make(map[int][]string, len(blocks))

	// Process outermost blocks first (reverse order since matchBlocks returns
	// inner blocks first due to stack-based matching).
	blockCount := len(blocks)
	for i := blockCount - 1; i >= 0; i-- {
		b := blocks[i]
		if tryConvertBlock(b, lines, filename, result, converted, replacements) {
			markConverted(b, converted)
		} else {
			// Block was flagged for review — mark all its directive lines as consumed
			// so convertLineSmart doesn't double-flag them. Body lines are passed through
			// with simple expression conversion.
			markFlaggedBlock(b, lines, converted, replacements)
		}
	}

	// Build output
	out := make([]string, 0, len(lines))
	lineCount := len(lines)
	for i := range lineCount {
		if rep, ok := replacements[i]; ok {
			out = append(out, rep...)
			continue
		}
		if converted[i] {
			continue
		}
		// Regular line — convert simple expressions
		out = append(out, convertLineSmart(lines[i], filename, i+1, result))
	}
	return strings.Join(out, "\n")
}

// markConverted marks all lines of a block as consumed.
func markConverted(b block, converted map[int]bool) {
	for i := b.startLine; i <= b.endLine; i++ {
		converted[i] = true
	}
}

// markFlaggedBlock marks directive lines of a flagged block as consumed,
// and passes body lines through with simple expression conversion.
func markFlaggedBlock(b block, lines []string, converted map[int]bool, replacements map[int][]string) {
	// Mark start/end/else directive lines as consumed (they'll be passed through as-is)
	directiveLines := map[int]bool{
		b.startLine: true,
		b.endLine:   true,
	}
	if -1 != b.elseLine {
		directiveLines[b.elseLine] = true
	}
	for _, eif := range b.elseIfs {
		directiveLines[eif.line] = true
	}

	// Pass all lines through as-is (directives stay, body gets simple conversion)
	outLines := make([]string, 0, b.endLine-b.startLine+1)
	for i := b.startLine; i <= b.endLine; i++ {
		if directiveLines[i] {
			outLines = append(outLines, lines[i])
		} else {
			outLines = append(outLines, convertSimpleExpressionsInLine(lines[i]))
		}
		converted[i] = true
	}
	replacements[b.startLine] = outLines
}

// tryConvertBlock attempts to convert a matched block to keramos syntax.
// Returns true if converted, false if it should be flagged for review.
func tryConvertBlock(b block, lines []string, filename string, result *MigrateResult, converted map[int]bool, replacements map[int][]string) bool {
	// Skip blocks whose lines are already consumed by an outer block conversion.
	for i := b.startLine; i <= b.endLine; i++ {
		if converted[i] {
			return false
		}
	}

	// Skip blocks with else-if chains (complex)
	if len(b.elseIfs) > 0 {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	// Check for complex content in condition
	if isComplexExpression(b.condition) {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	// Check body for complex content
	bodyStart := b.startLine + 1
	bodyEnd := b.endLine - 1
	if -1 != b.elseLine {
		bodyEnd = b.elseLine - 1
	}

	if hasComplexContent(lines, bodyStart, bodyEnd) || hasDollarVarRefs(lines, bodyStart, bodyEnd) {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	// If there's an else, check else body too
	if -1 != b.elseLine {
		elseBodyStart := b.elseLine + 1
		elseBodyEnd := b.endLine - 1
		if hasComplexContent(lines, elseBodyStart, elseBodyEnd) || hasDollarVarRefs(lines, elseBodyStart, elseBodyEnd) {
			flagBlockForReview(b, lines, filename, result)
			return false
		}
	}

	switch b.kind {
	case dirWith:
		return tryConvertWith(b, lines, filename, result, replacements)
	case dirIf:
		return tryConvertIf(b, lines, filename, result, replacements)
	case dirRange:
		return tryConvertRange(b, lines, filename, result, replacements)
	}
	return false
}

// tryConvertWith handles with blocks.
func tryConvertWith(b block, lines []string, filename string, result *MigrateResult, replacements map[int][]string) bool {
	bodyStart := b.startLine + 1
	bodyEnd := b.endLine - 1

	// Determine the with target
	condRef := convertCondition(b.condition)
	if "" == condRef {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	indent := extractIndent(lines[b.startLine])

	// Pattern: with + toYaml body (most common)
	if isWithToYamlBody(lines, bodyStart, bodyEnd) {
		replacements[b.startLine] = []string{
			indent + "$merge: ${" + condRef + "}",
		}
		return true
	}

	// Pattern: with wrapping a YAML key + toYaml body
	// e.g.:
	//   {{- with .Values.nodeSelector }}
	//   nodeSelector:
	//     {{- toYaml . | nindent 8 }}
	//   {{- end }}
	if !bodyHasNestedBlocks(lines, bodyStart, bodyEnd) && !isWithScalarBody(lines, bodyStart, bodyEnd) {
		return tryConvertWithBlock(b, lines, condRef, indent, replacements)
	}

	// Pattern: with wrapping a single scalar line
	if isWithScalarBody(lines, bodyStart, bodyEnd) {
		return tryConvertWithScalar(b, lines, condRef, indent, replacements)
	}

	flagBlockForReview(b, lines, filename, result)
	return false
}

// tryConvertWithBlock converts a with block that wraps YAML content (key + toYaml body).
func tryConvertWithBlock(b block, lines []string, condRef, indent string, replacements map[int][]string) bool {
	bodyStart := b.startLine + 1
	bodyEnd := b.endLine - 1

	bodyLines := make([]string, 0, bodyEnd-bodyStart+1)
	for i := bodyStart; i <= bodyEnd; i++ {
		line := lines[i]
		// Replace {{- toYaml . | nindent N }} with $merge reference
		if reToYamlLine.MatchString(line) {
			lineIndent := extractIndent(line)
			bodyLines = append(bodyLines, lineIndent+"$merge: ${"+condRef+"}")
			continue
		}
		// Replace {{ . }} or {{ . | quote }} dot-references with the condRef
		if reWithDotRef.MatchString(line) {
			replaced := reWithDotRef.ReplaceAllStringFunc(line, func(m string) string {
				return convertWithDotExpression(m, condRef)
			})
			bodyLines = append(bodyLines, replaced)
			continue
		}
		// Convert simple expressions in remaining lines
		bodyLines = append(bodyLines, convertSimpleExpressionsInLine(line))
	}

	outLines := make([]string, 0, len(bodyLines)+2)
	outLines = append(outLines, indent+"$if: ${"+condRef+"}")
	outLines = append(outLines, indent+"$then:")
	for _, bl := range bodyLines {
		outLines = append(outLines, indent+"  "+strings.TrimPrefix(bl, indent))
	}
	replacements[b.startLine] = outLines
	return true
}

// tryConvertWithScalar converts a with block that wraps a single scalar line.
func tryConvertWithScalar(b block, lines []string, condRef, indent string, replacements map[int][]string) bool {
	bodyStart := b.startLine + 1
	bodyEnd := b.endLine - 1

	var bodyLine string
	for i := bodyStart; i <= bodyEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if "" != trimmed {
			bodyLine = lines[i]
			break
		}
	}

	// Replace dot references in the body line
	if reWithDotRef.MatchString(bodyLine) {
		bodyLine = reWithDotRef.ReplaceAllStringFunc(bodyLine, func(m string) string {
			return convertWithDotExpression(m, condRef)
		})
	}
	bodyLine = convertSimpleExpressionsInLine(bodyLine)

	outLines := []string{
		indent + "$if: ${" + condRef + "}",
		indent + "$then:",
		indent + "  " + strings.TrimPrefix(bodyLine, indent),
	}
	replacements[b.startLine] = outLines
	return true
}

// convertWithDotExpression converts {{ . }} or {{ . | filter }} inside a with block
// to ${condRef} or ${condRef | filter}.
func convertWithDotExpression(expr, condRef string) string {
	inner := expr
	inner = strings.TrimPrefix(inner, "{{-")
	inner = strings.TrimPrefix(inner, "{{")
	inner = strings.TrimSuffix(inner, "-}}")
	inner = strings.TrimSuffix(inner, "}}")
	inner = strings.TrimSpace(inner)

	parts := splitPipe(inner)
	if 0 == len(parts) {
		return expr
	}

	// First part should be "."
	if "." != strings.TrimSpace(parts[0]) {
		return expr
	}

	if 1 == len(parts) {
		return "${" + condRef + "}"
	}

	filters := make([]string, 0, len(parts)-1)
	for _, p := range parts[1:] {
		filters = append(filters, convertFilter(strings.TrimSpace(p)))
	}
	return "${" + condRef + " | " + strings.Join(filters, " | ") + "}"
}

// tryConvertIf handles if/else blocks.
func tryConvertIf(b block, lines []string, filename string, result *MigrateResult, replacements map[int][]string) bool {
	condRef := convertCondition(b.condition)
	if "" == condRef {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	bodyStart := b.startLine + 1
	bodyEnd := b.endLine - 1
	if -1 != b.elseLine {
		bodyEnd = b.elseLine - 1
	}

	// Check for nested blocks in the body
	if bodyHasNestedBlocks(lines, bodyStart, bodyEnd) {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	if -1 != b.elseLine {
		elseStart := b.elseLine + 1
		elseEnd := b.endLine - 1
		if bodyHasNestedBlocks(lines, elseStart, elseEnd) {
			flagBlockForReview(b, lines, filename, result)
			return false
		}
	}

	indent := extractIndent(lines[b.startLine])

	// Simple if without else
	if -1 == b.elseLine {
		return tryConvertSimpleIf(b, lines, condRef, indent, bodyStart, bodyEnd, replacements)
	}

	// If/else
	return tryConvertIfElse(b, lines, condRef, indent, bodyStart, bodyEnd, replacements)
}

func tryConvertSimpleIf(b block, lines []string, condRef, indent string, bodyStart, bodyEnd int, replacements map[int][]string) bool {
	bodyLines := convertBodyLines(lines, bodyStart, bodyEnd)

	outLines := make([]string, 0, len(bodyLines)+1)
	outLines = append(outLines, indent+"$if: ${"+condRef+"}")

	// If there's only one body line and it looks like plain YAML, emit without $then
	if 1 == len(bodyLines) && !strings.HasPrefix(strings.TrimSpace(bodyLines[0]), "$") {
		outLines = append(outLines, bodyLines...)
	} else {
		outLines = append(outLines, indent+"$then:")
		for _, bl := range bodyLines {
			outLines = append(outLines, indent+"  "+strings.TrimPrefix(bl, indent))
		}
	}

	replacements[b.startLine] = outLines
	return true
}

func tryConvertIfElse(b block, lines []string, condRef, indent string, bodyStart, bodyEnd int, replacements map[int][]string) bool {
	elseStart := b.elseLine + 1
	elseEnd := b.endLine - 1

	thenLines := convertBodyLines(lines, bodyStart, bodyEnd)
	elseLines := convertBodyLines(lines, elseStart, elseEnd)

	outLines := make([]string, 0, len(thenLines)+len(elseLines)+3)
	outLines = append(outLines, indent+"$if: ${"+condRef+"}")
	outLines = append(outLines, indent+"$then:")
	for _, bl := range thenLines {
		outLines = append(outLines, indent+"  "+strings.TrimPrefix(bl, indent))
	}
	outLines = append(outLines, indent+"$else:")
	for _, bl := range elseLines {
		outLines = append(outLines, indent+"  "+strings.TrimPrefix(bl, indent))
	}

	replacements[b.startLine] = outLines
	return true
}

// tryConvertRange handles range blocks.
func tryConvertRange(b block, lines []string, filename string, result *MigrateResult, replacements map[int][]string) bool {
	bodyStart := b.startLine + 1
	bodyEnd := b.endLine - 1

	// Check for nested blocks
	if bodyHasNestedBlocks(lines, bodyStart, bodyEnd) {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	// Parse range expression
	rangeTarget := convertRangeTarget(b.condition)
	if "" == rangeTarget {
		flagBlockForReview(b, lines, filename, result)
		return false
	}

	indent := extractIndent(lines[b.startLine])

	// Convert body lines, replacing . references with item
	bodyLines := make([]string, 0, bodyEnd-bodyStart+1)
	for i := bodyStart; i <= bodyEnd; i++ {
		line := lines[i]
		line = convertRangeBodyLine(line)
		bodyLines = append(bodyLines, line)
	}

	outLines := make([]string, 0, len(bodyLines)+3)
	outLines = append(outLines, indent+"$each: ${"+rangeTarget+"}")
	outLines = append(outLines, indent+"$as: item")
	outLines = append(outLines, indent+"$yield:")
	for _, bl := range bodyLines {
		outLines = append(outLines, indent+"  "+strings.TrimPrefix(bl, indent))
	}

	replacements[b.startLine] = outLines
	return true
}

// convertRangeTarget parses a range expression and returns the keramos target.
func convertRangeTarget(cond string) string {
	cond = strings.TrimSpace(cond)

	// Handle range with key,value: range $k, $v := .Values.x
	if strings.Contains(cond, ":=") {
		return ""
	}

	if strings.HasPrefix(cond, ".Values.") {
		return "values." + cond[len(".Values."):]
	}

	return ""
}

// convertRangeBodyLine converts references inside a range body.
// . becomes item, .field becomes item.field.
var reDotField = regexp.MustCompile(`\{\{-?\s*(\.(\w+(\.\w+)*))\s*(\|[^}]*)?\s*-?\}\}`)
var reDotAlone = regexp.MustCompile(`\{\{-?\s*\.\s*(\|[^}]*)?\s*-?\}\}`)

func convertRangeBodyLine(line string) string {
	// Replace {{ .field }} with ${item.field}
	line = reDotField.ReplaceAllStringFunc(line, func(m string) string {
		inner := m
		inner = strings.TrimPrefix(inner, "{{-")
		inner = strings.TrimPrefix(inner, "{{")
		inner = strings.TrimSuffix(inner, "-}}")
		inner = strings.TrimSuffix(inner, "}}")
		inner = strings.TrimSpace(inner)

		parts := splitPipe(inner)
		if 0 == len(parts) {
			return m
		}

		ref := strings.TrimSpace(parts[0])

		// Skip .Values. and .Release. references — those are global
		if strings.HasPrefix(ref, ".Values.") || strings.HasPrefix(ref, ".Release.") ||
			strings.HasPrefix(ref, ".Chart.") {
			return convertExpression(m)
		}

		// .field -> item.field
		if strings.HasPrefix(ref, ".") && len(ref) > 1 {
			keramosRef := "item" + ref
			if 1 == len(parts) {
				return "${" + keramosRef + "}"
			}
			filters := make([]string, 0, len(parts)-1)
			for _, p := range parts[1:] {
				filters = append(filters, convertFilter(strings.TrimSpace(p)))
			}
			return "${" + keramosRef + " | " + strings.Join(filters, " | ") + "}"
		}
		return m
	})

	// Replace standalone {{ . }} with ${item}
	line = reDotAlone.ReplaceAllStringFunc(line, func(m string) string {
		inner := m
		inner = strings.TrimPrefix(inner, "{{-")
		inner = strings.TrimPrefix(inner, "{{")
		inner = strings.TrimSuffix(inner, "-}}")
		inner = strings.TrimSuffix(inner, "}}")
		inner = strings.TrimSpace(inner)

		parts := splitPipe(inner)
		if 0 == len(parts) {
			return m
		}

		ref := strings.TrimSpace(parts[0])
		if "." != ref {
			return m
		}

		if 1 == len(parts) {
			return "${item}"
		}
		filters := make([]string, 0, len(parts)-1)
		for _, p := range parts[1:] {
			filters = append(filters, convertFilter(strings.TrimSpace(p)))
		}
		return "${item | " + strings.Join(filters, " | ") + "}"
	})

	return line
}

// convertBodyLines processes lines in a block body, converting simple expressions.
func convertBodyLines(lines []string, from, to int) []string {
	out := make([]string, 0, to-from+1)
	for i := from; i <= to; i++ {
		if i >= len(lines) {
			break
		}
		out = append(out, convertSimpleExpressionsInLine(lines[i]))
	}
	return out
}

// convertSimpleExpressionsInLine converts inline Helm expressions but not control flow.
func convertSimpleExpressionsInLine(line string) string {
	// Auto-translate simple-form helm functions that keramos's runtime supports.
	if rewritten := rewriteSimpleHelmFunc(line); rewritten != line {
		return rewritten
	}

	// Handle standalone toYaml .Values.x | nindent N
	if m := reToYamlStandalone.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$merge: ${values." + m[1] + "}"
	}

	// Handle standalone include "name" . | nindent N
	if m := reIncludeStandalone.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$include: " + m[1]
	}

	// Handle standalone include "name" .
	if m := reIncludeLine.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$include: " + m[1]
	}

	return reSimpleExpr.ReplaceAllStringFunc(line, func(match string) string {
		return convertExpression(match)
	})
}

// convertLineSmart is like convertLine but doesn't flag already-handled directives.
func convertLineSmart(line, filename string, lineNum int, result *MigrateResult) string {
	trimmed := strings.TrimSpace(line)

	// Standalone toYaml
	if m := reToYamlStandalone.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$merge: ${values." + m[1] + "}"
	}

	// Standalone include with nindent
	if m := reIncludeStandalone.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$include: " + m[1]
	}

	// Standalone include without nindent
	if m := reIncludeLine.FindStringSubmatch(line); nil != m {
		indent := extractIndent(line)
		return indent + "$include: " + m[1]
	}

	// Flag tpl calls
	if reTpl.MatchString(trimmed) {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File: filename, Line: lineNum, Reason: "tpl call requires manual conversion", Original: trimmed,
		})
		return line
	}

	// Flag unmatched control flow (shouldn't happen if blocks matched, but safety)
	kind, _ := classifyLine(line)
	if dirNone != kind {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File: filename, Line: lineNum, Reason: "unmatched control flow directive", Original: trimmed,
		})
		return line
	}

	return reSimpleExpr.ReplaceAllStringFunc(line, func(match string) string {
		return convertExpression(match)
	})
}

// flagBlockForReview adds a single review item for the opening directive of a block.
func flagBlockForReview(b block, lines []string, filename string, result *MigrateResult) {
	kindName := "block"
	switch b.kind {
	case dirWith:
		kindName = "with block"
	case dirIf:
		kindName = "if block"
	case dirRange:
		kindName = "range block"
	}

	if b.startLine >= 0 && b.startLine < len(lines) {
		result.ManualReview = append(result.ManualReview, ReviewItem{
			File:     filename,
			Line:     b.startLine + 1,
			Reason:   kindName + " requires manual conversion",
			Original: strings.TrimSpace(lines[b.startLine]),
		})
	}
}
