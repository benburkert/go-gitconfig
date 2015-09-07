package gitconfig

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 1114112

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleGrammar
	ruleSection
	ruleValueLine
	ruleValue
	ruleIdentifier
	ruleWord
	ruleSpaceComment
	ruleComment
	ruleSpace
	ruleEndOfLine
	rulePegText
	ruleAction0
	ruleAction1
	ruleAction2
	ruleAction3

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"Grammar",
	"Section",
	"ValueLine",
	"Value",
	"Identifier",
	"Word",
	"SpaceComment",
	"Comment",
	"Space",
	"EndOfLine",
	"PegText",
	"Action0",
	"Action1",
	"Action2",
	"Action3",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next uint32, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(string(([]rune(buffer)[node.begin:node.end]))))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next uint32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: uint32(t.begin), end: uint32(t.end), next: uint32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = uint32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, uint32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(string(([]rune(buffer)[token.begin:token.end]))))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth uint32, index int) {
	t.tree[index] = token32{pegRule: rule, begin: uint32(begin), end: uint32(end), next: uint32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/*func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2 * len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}*/

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type config struct {
	sections   []*Section
	curSection *Section
	curKey     string

	Buffer string
	buffer []rune
	rules  [16]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range []rune(buffer) {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *config
}

func (e *parseError) Error() string {
	tokens, error := e.p.tokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *config) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *config) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *config) Execute() {
	buffer, _buffer, text, begin, end := p.Buffer, p.buffer, "", 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {

		case rulePegText:
			begin, end = int(token.begin), int(token.end)
			text = string(_buffer[begin:end])

		case ruleAction0:
			p.addSection(text)
		case ruleAction1:
			p.setID(text)
		case ruleAction2:
			p.setKey(text)
		case ruleAction3:
			p.addValue(text)

		}
	}
	_, _, _, _, _ = buffer, _buffer, text, begin, end
}

func (p *config) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens32{tree: make([]token32, math.MaxInt16)}
	position, depth, tokenIndex, buffer, _rules := uint32(0), uint32(0), 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin uint32) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	_rules = [...]func() bool{
		nil,
		/* 0 Grammar <- <(SpaceComment / Section)+> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				{
					position4, tokenIndex4, depth4 := position, tokenIndex, depth
					if !_rules[ruleSpaceComment]() {
						goto l5
					}
					goto l4
				l5:
					position, tokenIndex, depth = position4, tokenIndex4, depth4
					{
						position6 := position
						depth++
					l7:
						{
							position8, tokenIndex8, depth8 := position, tokenIndex, depth
							if !_rules[ruleSpace]() {
								goto l8
							}
							goto l7
						l8:
							position, tokenIndex, depth = position8, tokenIndex8, depth8
						}
						if buffer[position] != rune('[') {
							goto l0
						}
						position++
					l9:
						{
							position10, tokenIndex10, depth10 := position, tokenIndex, depth
							if !_rules[ruleSpace]() {
								goto l10
							}
							goto l9
						l10:
							position, tokenIndex, depth = position10, tokenIndex10, depth10
						}
						{
							position11 := position
							depth++
							if !_rules[ruleIdentifier]() {
								goto l0
							}
							depth--
							add(rulePegText, position11)
						}
						{
							add(ruleAction0, position)
						}
						{
							position13, tokenIndex13, depth13 := position, tokenIndex, depth
							if !_rules[ruleSpace]() {
								goto l13
							}
						l15:
							{
								position16, tokenIndex16, depth16 := position, tokenIndex, depth
								if !_rules[ruleSpace]() {
									goto l16
								}
								goto l15
							l16:
								position, tokenIndex, depth = position16, tokenIndex16, depth16
							}
							if buffer[position] != rune('"') {
								goto l13
							}
							position++
							{
								position17 := position
								depth++
								if !_rules[ruleIdentifier]() {
									goto l13
								}
								depth--
								add(rulePegText, position17)
							}
							{
								add(ruleAction1, position)
							}
							if buffer[position] != rune('"') {
								goto l13
							}
							position++
							goto l14
						l13:
							position, tokenIndex, depth = position13, tokenIndex13, depth13
						}
					l14:
					l19:
						{
							position20, tokenIndex20, depth20 := position, tokenIndex, depth
							if !_rules[ruleSpace]() {
								goto l20
							}
							goto l19
						l20:
							position, tokenIndex, depth = position20, tokenIndex20, depth20
						}
						if buffer[position] != rune(']') {
							goto l0
						}
						position++
						if !_rules[ruleSpaceComment]() {
							goto l0
						}
					l21:
						{
							position22, tokenIndex22, depth22 := position, tokenIndex, depth
							{
								position23 := position
								depth++
							l24:
								{
									position25, tokenIndex25, depth25 := position, tokenIndex, depth
									if !_rules[ruleSpace]() {
										goto l25
									}
									goto l24
								l25:
									position, tokenIndex, depth = position25, tokenIndex25, depth25
								}
								{
									position26 := position
									depth++
									if !_rules[ruleIdentifier]() {
										goto l22
									}
									depth--
									add(rulePegText, position26)
								}
								{
									add(ruleAction2, position)
								}
							l28:
								{
									position29, tokenIndex29, depth29 := position, tokenIndex, depth
									if !_rules[ruleSpace]() {
										goto l29
									}
									goto l28
								l29:
									position, tokenIndex, depth = position29, tokenIndex29, depth29
								}
								if buffer[position] != rune('=') {
									goto l22
								}
								position++
							l30:
								{
									position31, tokenIndex31, depth31 := position, tokenIndex, depth
									if !_rules[ruleSpace]() {
										goto l31
									}
									goto l30
								l31:
									position, tokenIndex, depth = position31, tokenIndex31, depth31
								}
								{
									position32 := position
									depth++
									{
										position33 := position
										depth++
										if !_rules[ruleWord]() {
											goto l22
										}
									l34:
										{
											position35, tokenIndex35, depth35 := position, tokenIndex, depth
											if !_rules[ruleSpace]() {
												goto l35
											}
										l36:
											{
												position37, tokenIndex37, depth37 := position, tokenIndex, depth
												if !_rules[ruleSpace]() {
													goto l37
												}
												goto l36
											l37:
												position, tokenIndex, depth = position37, tokenIndex37, depth37
											}
											if !_rules[ruleWord]() {
												goto l35
											}
											goto l34
										l35:
											position, tokenIndex, depth = position35, tokenIndex35, depth35
										}
										depth--
										add(ruleValue, position33)
									}
									depth--
									add(rulePegText, position32)
								}
								{
									add(ruleAction3, position)
								}
								if !_rules[ruleSpaceComment]() {
									goto l22
								}
								depth--
								add(ruleValueLine, position23)
							}
							goto l21
						l22:
							position, tokenIndex, depth = position22, tokenIndex22, depth22
						}
						depth--
						add(ruleSection, position6)
					}
				}
			l4:
			l2:
				{
					position3, tokenIndex3, depth3 := position, tokenIndex, depth
					{
						position39, tokenIndex39, depth39 := position, tokenIndex, depth
						if !_rules[ruleSpaceComment]() {
							goto l40
						}
						goto l39
					l40:
						position, tokenIndex, depth = position39, tokenIndex39, depth39
						{
							position41 := position
							depth++
						l42:
							{
								position43, tokenIndex43, depth43 := position, tokenIndex, depth
								if !_rules[ruleSpace]() {
									goto l43
								}
								goto l42
							l43:
								position, tokenIndex, depth = position43, tokenIndex43, depth43
							}
							if buffer[position] != rune('[') {
								goto l3
							}
							position++
						l44:
							{
								position45, tokenIndex45, depth45 := position, tokenIndex, depth
								if !_rules[ruleSpace]() {
									goto l45
								}
								goto l44
							l45:
								position, tokenIndex, depth = position45, tokenIndex45, depth45
							}
							{
								position46 := position
								depth++
								if !_rules[ruleIdentifier]() {
									goto l3
								}
								depth--
								add(rulePegText, position46)
							}
							{
								add(ruleAction0, position)
							}
							{
								position48, tokenIndex48, depth48 := position, tokenIndex, depth
								if !_rules[ruleSpace]() {
									goto l48
								}
							l50:
								{
									position51, tokenIndex51, depth51 := position, tokenIndex, depth
									if !_rules[ruleSpace]() {
										goto l51
									}
									goto l50
								l51:
									position, tokenIndex, depth = position51, tokenIndex51, depth51
								}
								if buffer[position] != rune('"') {
									goto l48
								}
								position++
								{
									position52 := position
									depth++
									if !_rules[ruleIdentifier]() {
										goto l48
									}
									depth--
									add(rulePegText, position52)
								}
								{
									add(ruleAction1, position)
								}
								if buffer[position] != rune('"') {
									goto l48
								}
								position++
								goto l49
							l48:
								position, tokenIndex, depth = position48, tokenIndex48, depth48
							}
						l49:
						l54:
							{
								position55, tokenIndex55, depth55 := position, tokenIndex, depth
								if !_rules[ruleSpace]() {
									goto l55
								}
								goto l54
							l55:
								position, tokenIndex, depth = position55, tokenIndex55, depth55
							}
							if buffer[position] != rune(']') {
								goto l3
							}
							position++
							if !_rules[ruleSpaceComment]() {
								goto l3
							}
						l56:
							{
								position57, tokenIndex57, depth57 := position, tokenIndex, depth
								{
									position58 := position
									depth++
								l59:
									{
										position60, tokenIndex60, depth60 := position, tokenIndex, depth
										if !_rules[ruleSpace]() {
											goto l60
										}
										goto l59
									l60:
										position, tokenIndex, depth = position60, tokenIndex60, depth60
									}
									{
										position61 := position
										depth++
										if !_rules[ruleIdentifier]() {
											goto l57
										}
										depth--
										add(rulePegText, position61)
									}
									{
										add(ruleAction2, position)
									}
								l63:
									{
										position64, tokenIndex64, depth64 := position, tokenIndex, depth
										if !_rules[ruleSpace]() {
											goto l64
										}
										goto l63
									l64:
										position, tokenIndex, depth = position64, tokenIndex64, depth64
									}
									if buffer[position] != rune('=') {
										goto l57
									}
									position++
								l65:
									{
										position66, tokenIndex66, depth66 := position, tokenIndex, depth
										if !_rules[ruleSpace]() {
											goto l66
										}
										goto l65
									l66:
										position, tokenIndex, depth = position66, tokenIndex66, depth66
									}
									{
										position67 := position
										depth++
										{
											position68 := position
											depth++
											if !_rules[ruleWord]() {
												goto l57
											}
										l69:
											{
												position70, tokenIndex70, depth70 := position, tokenIndex, depth
												if !_rules[ruleSpace]() {
													goto l70
												}
											l71:
												{
													position72, tokenIndex72, depth72 := position, tokenIndex, depth
													if !_rules[ruleSpace]() {
														goto l72
													}
													goto l71
												l72:
													position, tokenIndex, depth = position72, tokenIndex72, depth72
												}
												if !_rules[ruleWord]() {
													goto l70
												}
												goto l69
											l70:
												position, tokenIndex, depth = position70, tokenIndex70, depth70
											}
											depth--
											add(ruleValue, position68)
										}
										depth--
										add(rulePegText, position67)
									}
									{
										add(ruleAction3, position)
									}
									if !_rules[ruleSpaceComment]() {
										goto l57
									}
									depth--
									add(ruleValueLine, position58)
								}
								goto l56
							l57:
								position, tokenIndex, depth = position57, tokenIndex57, depth57
							}
							depth--
							add(ruleSection, position41)
						}
					}
				l39:
					goto l2
				l3:
					position, tokenIndex, depth = position3, tokenIndex3, depth3
				}
				depth--
				add(ruleGrammar, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 Section <- <(Space* '[' Space* <Identifier> Action0 (Space+ '"' <Identifier> Action1 '"')? Space* ']' SpaceComment ValueLine*)> */
		nil,
		/* 2 ValueLine <- <(Space* <Identifier> Action2 Space* '=' Space* <Value> Action3 SpaceComment)> */
		nil,
		/* 3 Value <- <(Word (Space+ Word)*)> */
		nil,
		/* 4 Identifier <- <((&('.') '.') | (&('@') '@') | (&('-') '-') | (&('_') '_') | (&('0' | '1' | '2' | '3' | '4' | '5' | '6' | '7' | '8' | '9') ([0-9] / [0-9])) | (&('A' | 'B' | 'C' | 'D' | 'E' | 'F' | 'G' | 'H' | 'I' | 'J' | 'K' | 'L' | 'M' | 'N' | 'O' | 'P' | 'Q' | 'R' | 'S' | 'T' | 'U' | 'V' | 'W' | 'X' | 'Y' | 'Z') [A-Z]) | (&('a' | 'b' | 'c' | 'd' | 'e' | 'f' | 'g' | 'h' | 'i' | 'j' | 'k' | 'l' | 'm' | 'n' | 'o' | 'p' | 'q' | 'r' | 's' | 't' | 'u' | 'v' | 'w' | 'x' | 'y' | 'z') [a-z]))+> */
		func() bool {
			position77, tokenIndex77, depth77 := position, tokenIndex, depth
			{
				position78 := position
				depth++
				{
					switch buffer[position] {
					case '.':
						if buffer[position] != rune('.') {
							goto l77
						}
						position++
						break
					case '@':
						if buffer[position] != rune('@') {
							goto l77
						}
						position++
						break
					case '-':
						if buffer[position] != rune('-') {
							goto l77
						}
						position++
						break
					case '_':
						if buffer[position] != rune('_') {
							goto l77
						}
						position++
						break
					case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
						{
							position82, tokenIndex82, depth82 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l83
							}
							position++
							goto l82
						l83:
							position, tokenIndex, depth = position82, tokenIndex82, depth82
							if c := buffer[position]; c < rune('0') || c > rune('9') {
								goto l77
							}
							position++
						}
					l82:
						break
					case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
						if c := buffer[position]; c < rune('A') || c > rune('Z') {
							goto l77
						}
						position++
						break
					default:
						if c := buffer[position]; c < rune('a') || c > rune('z') {
							goto l77
						}
						position++
						break
					}
				}

			l79:
				{
					position80, tokenIndex80, depth80 := position, tokenIndex, depth
					{
						switch buffer[position] {
						case '.':
							if buffer[position] != rune('.') {
								goto l80
							}
							position++
							break
						case '@':
							if buffer[position] != rune('@') {
								goto l80
							}
							position++
							break
						case '-':
							if buffer[position] != rune('-') {
								goto l80
							}
							position++
							break
						case '_':
							if buffer[position] != rune('_') {
								goto l80
							}
							position++
							break
						case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
							{
								position85, tokenIndex85, depth85 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l86
								}
								position++
								goto l85
							l86:
								position, tokenIndex, depth = position85, tokenIndex85, depth85
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l80
								}
								position++
							}
						l85:
							break
						case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l80
							}
							position++
							break
						default:
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l80
							}
							position++
							break
						}
					}

					goto l79
				l80:
					position, tokenIndex, depth = position80, tokenIndex80, depth80
				}
				depth--
				add(ruleIdentifier, position78)
			}
			return true
		l77:
			position, tokenIndex, depth = position77, tokenIndex77, depth77
			return false
		},
		/* 5 Word <- <(!((&('\n') '\n') | (&('\r') '\r') | (&('#') '#') | (&('\t') '\t') | (&(' ') ' ')) .)+> */
		func() bool {
			position87, tokenIndex87, depth87 := position, tokenIndex, depth
			{
				position88 := position
				depth++
				{
					position91, tokenIndex91, depth91 := position, tokenIndex, depth
					{
						switch buffer[position] {
						case '\n':
							if buffer[position] != rune('\n') {
								goto l91
							}
							position++
							break
						case '\r':
							if buffer[position] != rune('\r') {
								goto l91
							}
							position++
							break
						case '#':
							if buffer[position] != rune('#') {
								goto l91
							}
							position++
							break
						case '\t':
							if buffer[position] != rune('\t') {
								goto l91
							}
							position++
							break
						default:
							if buffer[position] != rune(' ') {
								goto l91
							}
							position++
							break
						}
					}

					goto l87
				l91:
					position, tokenIndex, depth = position91, tokenIndex91, depth91
				}
				if !matchDot() {
					goto l87
				}
			l89:
				{
					position90, tokenIndex90, depth90 := position, tokenIndex, depth
					{
						position93, tokenIndex93, depth93 := position, tokenIndex, depth
						{
							switch buffer[position] {
							case '\n':
								if buffer[position] != rune('\n') {
									goto l93
								}
								position++
								break
							case '\r':
								if buffer[position] != rune('\r') {
									goto l93
								}
								position++
								break
							case '#':
								if buffer[position] != rune('#') {
									goto l93
								}
								position++
								break
							case '\t':
								if buffer[position] != rune('\t') {
									goto l93
								}
								position++
								break
							default:
								if buffer[position] != rune(' ') {
									goto l93
								}
								position++
								break
							}
						}

						goto l90
					l93:
						position, tokenIndex, depth = position93, tokenIndex93, depth93
					}
					if !matchDot() {
						goto l90
					}
					goto l89
				l90:
					position, tokenIndex, depth = position90, tokenIndex90, depth90
				}
				depth--
				add(ruleWord, position88)
			}
			return true
		l87:
			position, tokenIndex, depth = position87, tokenIndex87, depth87
			return false
		},
		/* 6 SpaceComment <- <((&('\n' | '\r') EndOfLine) | (&('#') Comment) | (&('\t' | ' ') Space+))> */
		func() bool {
			position95, tokenIndex95, depth95 := position, tokenIndex, depth
			{
				position96 := position
				depth++
				{
					switch buffer[position] {
					case '\n', '\r':
						if !_rules[ruleEndOfLine]() {
							goto l95
						}
						break
					case '#':
						{
							position98 := position
							depth++
							if buffer[position] != rune('#') {
								goto l95
							}
							position++
						l99:
							{
								position100, tokenIndex100, depth100 := position, tokenIndex, depth
								{
									position101, tokenIndex101, depth101 := position, tokenIndex, depth
									if !_rules[ruleEndOfLine]() {
										goto l101
									}
									goto l100
								l101:
									position, tokenIndex, depth = position101, tokenIndex101, depth101
								}
								if !matchDot() {
									goto l100
								}
								goto l99
							l100:
								position, tokenIndex, depth = position100, tokenIndex100, depth100
							}
							if !_rules[ruleEndOfLine]() {
								goto l95
							}
							depth--
							add(ruleComment, position98)
						}
						break
					default:
						if !_rules[ruleSpace]() {
							goto l95
						}
					l102:
						{
							position103, tokenIndex103, depth103 := position, tokenIndex, depth
							if !_rules[ruleSpace]() {
								goto l103
							}
							goto l102
						l103:
							position, tokenIndex, depth = position103, tokenIndex103, depth103
						}
						break
					}
				}

				depth--
				add(ruleSpaceComment, position96)
			}
			return true
		l95:
			position, tokenIndex, depth = position95, tokenIndex95, depth95
			return false
		},
		/* 7 Comment <- <('#' (!EndOfLine .)* EndOfLine)> */
		nil,
		/* 8 Space <- <(' ' / '\t')> */
		func() bool {
			position105, tokenIndex105, depth105 := position, tokenIndex, depth
			{
				position106 := position
				depth++
				{
					position107, tokenIndex107, depth107 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l108
					}
					position++
					goto l107
				l108:
					position, tokenIndex, depth = position107, tokenIndex107, depth107
					if buffer[position] != rune('\t') {
						goto l105
					}
					position++
				}
			l107:
				depth--
				add(ruleSpace, position106)
			}
			return true
		l105:
			position, tokenIndex, depth = position105, tokenIndex105, depth105
			return false
		},
		/* 9 EndOfLine <- <(('\r' '\n') / '\n' / '\r')> */
		func() bool {
			position109, tokenIndex109, depth109 := position, tokenIndex, depth
			{
				position110 := position
				depth++
				{
					position111, tokenIndex111, depth111 := position, tokenIndex, depth
					if buffer[position] != rune('\r') {
						goto l112
					}
					position++
					if buffer[position] != rune('\n') {
						goto l112
					}
					position++
					goto l111
				l112:
					position, tokenIndex, depth = position111, tokenIndex111, depth111
					if buffer[position] != rune('\n') {
						goto l113
					}
					position++
					goto l111
				l113:
					position, tokenIndex, depth = position111, tokenIndex111, depth111
					if buffer[position] != rune('\r') {
						goto l109
					}
					position++
				}
			l111:
				depth--
				add(ruleEndOfLine, position110)
			}
			return true
		l109:
			position, tokenIndex, depth = position109, tokenIndex109, depth109
			return false
		},
		nil,
		/* 12 Action0 <- <{ p.addSection(text) }> */
		nil,
		/* 13 Action1 <- <{ p.setID(text) }> */
		nil,
		/* 14 Action2 <- <{ p.setKey(text) }> */
		nil,
		/* 15 Action3 <- <{ p.addValue(text) }> */
		nil,
	}
	p.rules = _rules
}
