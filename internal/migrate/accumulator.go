package migrate

import (
	"strconv"
	"strings"
)

type letBinding struct {
	name  string
	lines []string
}

func (e *emitter) collectAccumulators(nodes []node, sc *scope) []node {
	rest := make([]node, 0, len(nodes))
	for _, n := range nodes {
		blk, isBlock := n.(*blockNode)
		if !isBlock || "range" != blk.kind || 1 != len(blk.branches) {
			rest = append(rest, n)
			continue
		}
		binding, ok := e.accumulatorBinding(blk, sc)
		if !ok {
			rest = append(rest, n)
			continue
		}
		e.lets = append(e.lets, binding)
		sc.set("$"+binding.name, binding.name)
	}
	return rest
}

func (e *emitter) accumulatorBinding(blk *blockNode, sc *scope) (letBinding, bool) {
	decls, collection := splitRangeDecl(blk.branches[0].cond)
	target, err := compileExpression(collection, sc)
	if nil != err {
		return letBinding{}, false
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

	name, cond, valueExpr, ok := appendedValue(blk.branches[0].body, inner, e)
	if !ok {
		return letBinding{}, false
	}

	lines := []string{"$each: ${" + target + "}", "$as: " + loopVar}
	if "" == cond {
		lines = append(lines, "$yield: ${"+valueExpr+"}")
	} else {
		lines = append(lines,
			"$yield:",
			"  $if: ${"+cond+"}",
			"  $then: ${"+valueExpr+"}")
	}
	return letBinding{name: strings.TrimPrefix(name, "$"), lines: lines}, true
}

func appendedValue(nodes []node, sc *scope, e *emitter) (string, string, string, bool) {
	cond := ""
	name := ""
	value := ""

	for _, n := range nodes {
		switch cur := n.(type) {
		case *textNode:
			if "" != strings.TrimSpace(cur.raw) {
				return "", "", "", false
			}
		case *actionNode:
			if !isAssignment(cur.body) {
				return "", "", "", false
			}
			if target, expr, ok := appendAssignment(cur.body, sc); ok {
				if "" != name {
					return "", "", "", false
				}
				name, value = target, expr
				continue
			}
			e.bindVariable(cur, sc)
		case *blockNode:
			if "if" != cur.kind || 1 != len(cur.branches) || "" != cond {
				return "", "", "", false
			}
			compiled, err := compileExpression(cur.branches[0].cond, sc)
			if nil != err {
				return "", "", "", false
			}
			inner, innerCond, innerValue, ok := appendedValue(cur.branches[0].body, sc.child(""), e)
			if !ok || "" != innerCond {
				return "", "", "", false
			}
			cond, name, value = compiled, inner, innerValue
		default:
			return "", "", "", false
		}
	}
	if "" == name || "" == value {
		return "", "", "", false
	}
	return name, cond, value, true
}

func appendAssignment(body string, sc *scope) (string, string, bool) {
	name, rhs, reassign := splitAssignment(body)
	if !reassign {
		return "", "", false
	}
	fields := strings.Fields(rhs)
	if 3 > len(fields) || "append" != fields[0] || fields[1] != name {
		return "", "", false
	}
	argument := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rhs), "append "+name))
	expr, err := compileExpression(argument, sc)
	if nil != err {
		return "", "", false
	}
	return name, expr, true
}

func letLines(bindings []letBinding, indent string) []string {
	if 0 == len(bindings) {
		return nil
	}
	out := []string{indent + "$let:"}
	for _, binding := range bindings {
		out = append(out, indent+"  "+binding.name+":")
		for _, line := range binding.lines {
			out = append(out, indent+"    "+line)
		}
	}
	return append(out, indent+"$in:")
}
