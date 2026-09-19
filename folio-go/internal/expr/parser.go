package expr

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxSourceBytes = 64 * 1024
const maxASTNodes = 4096
const maxCallDepth = 64

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokString
	tokNumber
)

type token struct {
	kind       tokenKind
	text       string
	start, end int
}
type parser struct {
	src               string
	pos, depth, nodes int
	boundaryEnd       int
}

func Parse(src string) (Expr, error) {
	if len(src) > maxSourceBytes {
		return nil, at(0, fmt.Errorf("expression exceeds %d source bytes", maxSourceBytes))
	}
	if strings.TrimSpace(src) == "" {
		return nil, at(0, fmt.Errorf("empty expression"))
	}
	// Trim only the boundaries, retaining the original prefix for byte offsets.
	start := len(src) - len(strings.TrimLeftFunc(src, unicode.IsSpace))
	bounded := strings.TrimRightFunc(src, unicode.IsSpace)
	p := &parser{src: src, pos: start, boundaryEnd: len(bounded)}
	e, err := p.expression(0)
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.pos != len(p.src) {
		return nil, at(p.pos, fmt.Errorf("unexpected trailing content %q", src[p.pos:]))
	}
	if err := preflight(e); err != nil {
		return nil, err
	}
	return e, nil
}
func isIdentStart(c byte) bool { return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
func isIdentCont(c byte) bool  { return isIdentStart(c) || isDigit(c) }
func isDigit(c byte) bool      { return c >= '0' && c <= '9' }
func (p *parser) skipWS() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t') {
		p.pos++
	}
	// Only the outer suffix accepts general Unicode whitespace; interior
	// token whitespace retains the existing grammar. EOF remains source-relative.
	if p.boundaryEnd > 0 && p.pos >= p.boundaryEnd {
		p.pos = len(p.src)
	}
}
func (p *parser) take(s string) bool {
	p.skipWS()
	if strings.HasPrefix(p.src[p.pos:], s) {
		p.pos += len(s)
		return true
	}
	return false
}
func (p *parser) fail(message string) error { return at(p.pos, fmt.Errorf("%s", message)) }
func (p *parser) expression(min int) (Expr, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxCallDepth {
		return nil, p.fail("expression nesting exceeds depth 64")
	}
	p.skipWS()
	start := p.pos
	left, err := p.atom()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		opPos := p.pos
		op, level := p.operator()
		if op == "" || level < min {
			break
		}
		p.pos += len(op)
		if op == "?" {
			yes, err := p.expression(0)
			if err != nil {
				return nil, err
			}
			if !p.take(":") {
				return nil, p.fail("expected ':' in conditional expression")
			}
			no, err := p.expression(level)
			if err != nil {
				return nil, err
			}
			left = &ConditionalExpr{Condition: left, Then: yes, Else: no, Raw: p.src[start:p.pos], Offset: opPos}
		} else {
			right, err := p.expression(level + 1)
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right, Raw: p.src[start:p.pos], Offset: opPos}
			p.skipWS()
			_, nextLevel := p.operator()
			if (level == 2 || level == 3) && nextLevel == level {
				return nil, p.fail("repeated unparenthesized comparisons are not supported")
			}
		}
		p.nodes++
		if p.nodes > maxASTNodes {
			return nil, p.fail("expression exceeds 4096 nodes")
		}
	}
	return left, nil
}
func (p *parser) operator() (string, int) {
	rest := p.src[p.pos:]
	for _, op := range []string{"!=", ">=", "<=", ">", "<", "+", "-", "*", "/", "%", "?"} {
		if strings.HasPrefix(rest, op) {
			switch op {
			case "?":
				return op, 1
			case "!=":
				return op, 2
			case ">", "<", ">=", "<=":
				return op, 3
			case "+", "-":
				return op, 4
			default:
				return op, 5
			}
		}
	}
	return "", -1
}
func (p *parser) atom() (Expr, error) {
	p.nodes++
	if p.nodes > maxASTNodes {
		return nil, p.fail("expression exceeds 4096 nodes")
	}
	p.skipWS()
	start := p.pos
	if start == len(p.src) {
		return nil, p.fail("unexpected end of expression")
	}
	c := p.src[start]
	if c == '+' || c == '-' {
		// An adjacent negative number remains one signed JSON literal, including
		// MinInt64. Negating a grouped MinInt64 is an arithmetic overflow.
		if c == '-' && start+1 < len(p.src) && isDigit(p.src[start+1]) {
			return p.number()
		}
		p.pos++
		operand, err := p.expression(6)
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: string(c), Operand: operand, Raw: p.src[start:p.pos], Offset: start}, nil
	}
	if isDigit(c) {
		return p.number()
	}
	if c == '"' {
		tok, err := p.lexString()
		if err != nil {
			return nil, at(start, err)
		}
		p.pos = tok.end
		return &StringLit{Value: tok.text[1 : len(tok.text)-1], Raw: tok.text, Offset: start}, nil
	}
	if c == '(' {
		p.pos++
		inner, err := p.expression(0)
		if err != nil {
			return nil, err
		}
		if !p.take(")") {
			return nil, p.fail("expected ')' in grouped expression")
		}
		return &GroupExpr{Inner: inner, Raw: p.src[start:p.pos], Offset: start}, nil
	}
	if isIdentStart(c) {
		p.pos++
		for p.pos < len(p.src) && isIdentCont(p.src[p.pos]) {
			p.pos++
		}
		name := p.src[start:p.pos]
		if name == "true" || name == "false" || name == "null" {
			end := p.pos
			p.skipWS()
			if p.pos < len(p.src) && (p.src[p.pos] == '.' || p.src[p.pos] == '(') {
				return nil, p.fail("literal keywords cannot be path roots or functions")
			}
			p.pos = end
			if name == "null" {
				return &NullLit{Raw: name, Offset: start}, nil
			}
			return &BoolLit{Value: name == "true", Raw: name, Offset: start}, nil
		}
		if p.pos < len(p.src) && p.src[p.pos] == '(' {
			p.pos++
			var args []Expr
			p.skipWS()
			if !p.take(")") {
				for {
					arg, err := p.expression(0)
					if err != nil {
						return nil, err
					}
					args = append(args, arg)
					if p.take(")") {
						break
					}
					if !p.take(",") {
						return nil, p.fail("expected ',' or ')' in function call")
					}
				}
			}
			return &CallExpr{Name: name, Args: args, Raw: p.src[start:p.pos], Offset: start}, nil
		}
		segments := []string{name}
		for p.pos < len(p.src) && p.src[p.pos] == '.' {
			p.pos++
			segStart := p.pos
			if p.pos == len(p.src) || !isIdentStart(p.src[p.pos]) {
				if p.pos == len(p.src) {
					return nil, p.fail("expected identifier after '.', got end of expression")
				}
				return nil, p.fail("expected identifier after '.'")
			}
			p.pos++
			for p.pos < len(p.src) && isIdentCont(p.src[p.pos]) {
				p.pos++
			}
			segments = append(segments, p.src[segStart:p.pos])
		}
		return &PathExpr{Segments: segments, Raw: p.src[start:p.pos], Offset: start}, nil
	}
	if c == '\n' || c == '\r' {
		return nil, p.fail("unexpected newline (only spaces and tabs are permitted here)")
	}
	r, _ := utf8.DecodeRuneInString(p.src[start:])
	return nil, p.fail(fmt.Sprintf("unexpected character %q", r))
}
func (p *parser) number() (Expr, error) {
	start := p.pos
	tok, err := p.lexNumber()
	if err != nil {
		return nil, at(start, err)
	}
	p.pos = tok.end
	return &NumberLit{Literal: tok.text, Raw: tok.text, Offset: start}, nil
}

func (p *parser) lexString() (token, error) {
	start := p.pos
	i := start + 1 // past the opening quote
	for i < len(p.src) && p.src[i] != '"' {
		if p.src[i] == '\\' && i+1 < len(p.src) && p.src[i+1] == '"' {
			return token{}, fmt.Errorf(`string literals do not support escape sequences (found \" at position %d): a quote cannot appear inside a string literal`, i)
		}
		i++
	}
	if i >= len(p.src) {
		return token{}, fmt.Errorf("unterminated string literal starting at position %d", start)
	}
	end := i + 1 // past the closing quote
	return token{kind: tokString, text: p.src[start:end], start: start, end: end}, nil
}

// lexNumber lexes a JSON number literal: -? digits (.digits)?
// ([eE][+-]?digits)? — the same shape
// internal/template.SplitJSONNumber accepts (NewDecimal, decimal.go,
// is what eventually consumes it). The integer part follows JSON's own
// grammar exactly (RFC 8259: "0" or a non-zero digit followed by more
// digits — no other leading zero is legal): "01"/"007"/"-01" are
// rejected here (QA Finding 7, Major), rather than being silently
// accepted by the parser and then silently normalised by
// SplitJSONNumber downstream (which explicitly trusts encoding/json's
// own upstream grammar check and performs none of its own), which is
// what ast.go's own doc comment on NumberLit already claims this
// grammar is.
func (p *parser) lexNumber() (token, error) {
	start := p.pos
	i := start
	if i < len(p.src) && p.src[i] == '-' {
		i++
	}
	digitsStart := i
	for i < len(p.src) && isDigit(p.src[i]) {
		i++
	}
	if i == digitsStart {
		return token{}, fmt.Errorf("invalid number literal at position %d", start)
	}
	if i-digitsStart > 1 && p.src[digitsStart] == '0' {
		return token{}, fmt.Errorf("invalid number literal (leading zero not allowed) at position %d", start)
	}
	if i < len(p.src) && p.src[i] == '.' {
		i++
		fracStart := i
		for i < len(p.src) && isDigit(p.src[i]) {
			i++
		}
		if i == fracStart {
			return token{}, fmt.Errorf("invalid number literal (digits expected after '.') at position %d", start)
		}
	}
	if i < len(p.src) && (p.src[i] == 'e' || p.src[i] == 'E') {
		i++
		if i < len(p.src) && (p.src[i] == '+' || p.src[i] == '-') {
			i++
		}
		expStart := i
		for i < len(p.src) && isDigit(p.src[i]) {
			i++
		}
		if i == expStart {
			return token{}, fmt.Errorf("invalid number literal (digits expected in exponent) at position %d", start)
		}
	}
	return token{kind: tokNumber, text: p.src[start:i], start: start, end: i}, nil
}
