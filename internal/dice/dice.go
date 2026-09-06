package dice

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Result is one evaluated # expression. It is evidence, not canon.
type Result struct {
	Label      string
	Expression string
	Total      int
	Detail     string
	Rolls      []int
}

// RNG yields a die face in [1, sides].
type RNG func(sides int) int

// DefaultRNG uses math/rand with a time-seeded source.
func DefaultRNG() RNG {
	source := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(sides int) int {
		if sides < 1 {
			return 0
		}
		return source.Intn(sides) + 1
	}
}

// FixedRNG cycles through faces for deterministic tests.
func FixedRNG(faces ...int) RNG {
	index := 0
	return func(sides int) int {
		if len(faces) == 0 || sides < 1 {
			return 0
		}
		face := faces[index%len(faces)]
		index++
		if face < 1 {
			face = 1
		}
		if face > sides {
			face = sides
		}
		return face
	}
}

// Evaluate parses and rolls a single expression such as "d20+5", "2d6+3",
// or "10+2*3". Optional labels belong outside this call ("damage").
func Evaluate(expression string, rng RNG) (Result, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return Result{}, fmt.Errorf("empty expression")
	}
	if rng == nil {
		rng = DefaultRNG()
	}
	tokens, err := tokenize(expression)
	if err != nil {
		return Result{}, err
	}
	expanded, rolls, err := expandDice(tokens, rng)
	if err != nil {
		return Result{}, err
	}
	total, err := evalTokens(expanded)
	if err != nil {
		return Result{}, err
	}
	detail := formatDetail(expanded, total)
	return Result{
		Expression: expression,
		Total:      total,
		Detail:     detail,
		Rolls:      rolls,
	}, nil
}

// Match is one # expression discovered in transcript text.
type Match struct {
	Label      string
	Expression string
	Raw        string
}

// Extract finds # expressions in transcript text, skipping #random/#location.
func Extract(text string) []Match {
	var found []Match
	for index := 0; index < len(text); index++ {
		if text[index] != '#' {
			continue
		}
		if index > 0 {
			prev := rune(text[index-1])
			if unicode.IsLetter(prev) || unicode.IsDigit(prev) {
				continue
			}
		}
		rest := text[index+1:]
		lower := strings.ToLower(rest)
		if strings.HasPrefix(lower, "random") && commandBoundary(rest, len("random")) {
			continue
		}
		if strings.HasPrefix(lower, "location") && commandBoundary(rest, len("location")) {
			continue
		}
		label, expr, raw, ok := parseTaggedExpression(rest)
		if !ok {
			continue
		}
		found = append(found, Match{Label: label, Expression: expr, Raw: "#" + raw})
		index += len(raw)
	}
	return found
}

func commandBoundary(rest string, commandLen int) bool {
	if len(rest) == commandLen {
		return true
	}
	r := rune(rest[commandLen])
	return unicode.IsSpace(r) || r == 0
}

func parseTaggedExpression(rest string) (label, expression, raw string, ok bool) {
	trimmedStart := 0
	for trimmedStart < len(rest) && (rest[trimmedStart] == ' ' || rest[trimmedStart] == '\t') {
		trimmedStart++
	}
	body := rest[trimmedStart:]
	if body == "" {
		return "", "", "", false
	}

	// #damage 2d6+3  or  #d20+5  or  #10+2
	if startsExpression(body) {
		expr, consumed := readExpression(body)
		if consumed == 0 {
			return "", "", "", false
		}
		return "", expr, rest[:trimmedStart+consumed], true
	}

	labelEnd := 0
	for labelEnd < len(body) {
		r := rune(body[labelEnd])
		if unicode.IsLetter(r) || r == '_' {
			labelEnd++
			continue
		}
		break
	}
	if labelEnd == 0 {
		return "", "", "", false
	}
	label = body[:labelEnd]
	after := body[labelEnd:]
	space := 0
	for space < len(after) && (after[space] == ' ' || after[space] == '\t') {
		space++
	}
	after = after[space:]
	if !startsExpression(after) {
		return "", "", "", false
	}
	expr, consumed := readExpression(after)
	if consumed == 0 {
		return "", "", "", false
	}
	raw = rest[:trimmedStart+labelEnd+space+consumed]
	return label, expr, raw, true
}

func startsExpression(s string) bool {
	if s == "" {
		return false
	}
	r := rune(s[0])
	if unicode.IsDigit(r) || r == '(' {
		return true
	}
	if r == 'd' || r == 'D' {
		if len(s) < 2 {
			return false
		}
		return unicode.IsDigit(rune(s[1]))
	}
	return false
}

func readExpression(s string) (string, int) {
	depth := 0
	consumed := 0
	for consumed < len(s) {
		r := rune(s[consumed])
		switch {
		case r == '(':
			depth++
		case r == ')':
			if depth == 0 {
				return strings.TrimSpace(s[:consumed]), consumed
			}
			depth--
		case unicode.IsLetter(r) && !(r == 'd' || r == 'D'):
			return strings.TrimSpace(s[:consumed]), consumed
		case unicode.IsSpace(r) && depth == 0:
			return strings.TrimSpace(s[:consumed]), consumed
		case r == ',' || r == ';' || r == '.' || r == '!' || r == '?' || r == ':':
			if depth == 0 {
				return strings.TrimSpace(s[:consumed]), consumed
			}
		}
		consumed++
	}
	return strings.TrimSpace(s[:consumed]), consumed
}

type tokenKind int

const (
	tokNumber tokenKind = iota
	tokDice
	tokOp
	tokLParen
	tokRParen
)

type token struct {
	kind    tokenKind
	value   string
	display string
	count   int
	sides   int
}

func tokenize(expression string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(expression) {
		r := rune(expression[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		switch {
		case r == '(':
			tokens = append(tokens, token{kind: tokLParen, value: "(", display: "("})
			i++
		case r == ')':
			tokens = append(tokens, token{kind: tokRParen, value: ")", display: ")"})
			i++
		case r == '+' || r == '-' || r == '*' || r == '/':
			tokens = append(tokens, token{kind: tokOp, value: string(r), display: string(r)})
			i++
		case r == 'd' || r == 'D' || unicode.IsDigit(r):
			tok, next, err := readDiceOrNumber(expression, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		default:
			return nil, fmt.Errorf("unexpected %q in %q", string(r), expression)
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty expression")
	}
	return tokens, nil
}

func readDiceOrNumber(expression string, start int) (token, int, error) {
	i := start
	count := 0
	hasCount := false
	for i < len(expression) && unicode.IsDigit(rune(expression[i])) {
		hasCount = true
		count = count*10 + int(expression[i]-'0')
		i++
	}
	if i < len(expression) && (expression[i] == 'd' || expression[i] == 'D') {
		i++
		if i >= len(expression) || !unicode.IsDigit(rune(expression[i])) {
			return token{}, start, fmt.Errorf("dice sides required in %q", expression[start:])
		}
		sides := 0
		for i < len(expression) && unicode.IsDigit(rune(expression[i])) {
			sides = sides*10 + int(expression[i]-'0')
			i++
		}
		if !hasCount {
			count = 1
		}
		if count < 1 || sides < 1 {
			return token{}, start, fmt.Errorf("invalid dice %q", expression[start:i])
		}
		raw := expression[start:i]
		return token{kind: tokDice, value: raw, display: raw, count: count, sides: sides}, i, nil
	}
	if !hasCount {
		return token{}, start, fmt.Errorf("expected number in %q", expression[start:])
	}
	raw := strconv.Itoa(count)
	return token{kind: tokNumber, value: raw, display: raw}, i, nil
}

func expandDice(tokens []token, rng RNG) ([]token, []int, error) {
	var expanded []token
	var rolls []int
	for _, tok := range tokens {
		if tok.kind != tokDice {
			expanded = append(expanded, tok)
			continue
		}
		sum := 0
		parts := make([]string, 0, tok.count)
		for face := 0; face < tok.count; face++ {
			roll := rng(tok.sides)
			rolls = append(rolls, roll)
			sum += roll
			parts = append(parts, strconv.Itoa(roll))
		}
		display := "[" + strings.Join(parts, "+") + "]"
		expanded = append(expanded, token{
			kind:    tokNumber,
			value:   strconv.Itoa(sum),
			display: display,
		})
	}
	return expanded, rolls, nil
}

func formatDetail(tokens []token, total int) string {
	var builder strings.Builder
	for _, tok := range tokens {
		if tok.display != "" {
			builder.WriteString(tok.display)
		} else {
			builder.WriteString(tok.value)
		}
	}
	builder.WriteString(" = ")
	builder.WriteString(strconv.Itoa(total))
	return builder.String()
}

func evalTokens(tokens []token) (int, error) {
	values, ops, err := toRPN(tokens)
	if err != nil {
		return 0, err
	}
	_ = ops
	return evalRPN(values)
}

func toRPN(tokens []token) ([]token, []token, error) {
	var output []token
	var stack []token
	precedence := map[string]int{"+": 1, "-": 1, "*": 2, "/": 2}
	unaryMinus := true
	for _, tok := range tokens {
		switch tok.kind {
		case tokNumber:
			output = append(output, tok)
			unaryMinus = false
		case tokLParen:
			stack = append(stack, tok)
			unaryMinus = true
		case tokRParen:
			for len(stack) > 0 && stack[len(stack)-1].kind != tokLParen {
				output = append(output, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			if len(stack) == 0 {
				return nil, nil, fmt.Errorf("mismatched parentheses")
			}
			stack = stack[:len(stack)-1]
			unaryMinus = false
		case tokOp:
			if tok.value == "-" && unaryMinus {
				output = append(output, token{kind: tokNumber, value: "0"})
			}
			for len(stack) > 0 && stack[len(stack)-1].kind == tokOp &&
				precedence[stack[len(stack)-1].value] >= precedence[tok.value] {
				output = append(output, stack[len(stack)-1])
				stack = stack[:len(stack)-1]
			}
			stack = append(stack, tok)
			unaryMinus = true
		default:
			return nil, nil, fmt.Errorf("unexpected token %q", tok.value)
		}
	}
	for len(stack) > 0 {
		if stack[len(stack)-1].kind == tokLParen {
			return nil, nil, fmt.Errorf("mismatched parentheses")
		}
		output = append(output, stack[len(stack)-1])
		stack = stack[:len(stack)-1]
	}
	return output, nil, nil
}

func evalRPN(tokens []token) (int, error) {
	var stack []int
	for _, tok := range tokens {
		switch tok.kind {
		case tokNumber:
			n, err := strconv.Atoi(tok.value)
			if err != nil {
				return 0, err
			}
			stack = append(stack, n)
		case tokOp:
			if len(stack) < 2 {
				return 0, fmt.Errorf("invalid expression")
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-2]
			switch tok.value {
			case "+":
				stack = append(stack, a+b)
			case "-":
				stack = append(stack, a-b)
			case "*":
				stack = append(stack, a*b)
			case "/":
				if b == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				stack = append(stack, a/b)
			default:
				return 0, fmt.Errorf("unknown operator %q", tok.value)
			}
		default:
			return 0, fmt.Errorf("invalid rpn token %q", tok.value)
		}
	}
	if len(stack) != 1 {
		return 0, fmt.Errorf("invalid expression")
	}
	return stack[0], nil
}
