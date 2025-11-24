package parsan

import (
	"crypto/rand"
	"fmt"
	"maps"
	"math/big"
	"slices"
	"strings"
)

func init() {
	closedCheckedChan = make(chan Checked)
	close(closedCheckedChan)
}

var closedCheckedChan chan Checked

type namedPathNode struct {
	ruleName string
	input    string
}

type parseContext struct {
	namedPath []namedPathNode
}

func newParseContext() *parseContext {
	return &parseContext{
		namedPath: []namedPathNode{},
	}
}

func (pc *parseContext) hasPathNode(node namedPathNode) bool {
	return slices.ContainsFunc(pc.namedPath, func(np namedPathNode) bool {
		return np.input == node.input && np.ruleName == node.ruleName
	})
}

func (pc *parseContext) appendPathNode(node namedPathNode) {
	pc.namedPath = append(pc.namedPath, node)
}

func (pc *parseContext) clone() *parseContext {
	return &parseContext{
		namedPath: slices.Clone(pc.namedPath),
	}
}

type Checked struct {
	Suggestion string
	ToParse    string
}

type RuleWithMaxLength interface {
	MaxLength() int
}

func ParseAndSanitize(in string, rule Type) []string {
	if mlRule, ok := rule.(RuleWithMaxLength); ok {
		if len(in) > mlRule.MaxLength() {
			in = in[:mlRule.MaxLength()]
		}
	}
	checked := rule.validate(newParseContext(), in)
	suggestions := map[string]struct{}{}
	for ch := range checked {
		if len(ch.ToParse) == 0 {
			if mlRule, ok := rule.(RuleWithMaxLength); ok {
				if mlRule.MaxLength() < len(ch.Suggestion) {

					continue
				}
			}
			suggestions[ch.Suggestion] = struct{}{}
		}
	}
	return slices.SortedStableFunc(
		maps.Keys(suggestions),
		func(a, b string) int {
			lena := len(a)
			lenb := len(b)
			switch {
			case lena < lenb:
				return 1
			case lena > lenb:
				return -1
			default:
				return strings.Compare(a, b)
			}
		},
	)
}

type Type interface {
	validate(*parseContext, string) <-chan Checked
	WithSuggestionFunc(SuggestionFunc) Type
}

type terminal struct {
	s string
}

var _ Type = &terminal{}

func Terminal(s string) Type {
	return &terminal{
		s: s,
	}
}

func (t *terminal) validate(ctx *parseContext, in string) <-chan Checked {
	out := make(chan Checked)
	go func() {
		defer close(out)
		if remaining, matches := strings.CutPrefix(in, t.s); matches {
			out <- Checked{
				Suggestion: t.s,
				ToParse:    remaining,
			}
		}
	}()
	return out
}

func (t *terminal) WithSuggestionFunc(fn SuggestionFunc) Type {
	panic("not implemented")
}

type rangeType struct {
	start     rune
	end       rune
	suggestFn SuggestionFunc
}

var _ Type = &rangeType{}

type SuggestionFunc func(string) []*Checked

func MergeSuggestionFuncs(fns ...SuggestionFunc) SuggestionFunc {
	return func(in string) []*Checked {
		checked := make([]*Checked, 0)
		for _, fn := range fns {
			checked = append(checked, fn(in)...)
		}
		return checked
	}
}

func SuggestConstRune(r rune) SuggestionFunc {
	return ReplaceFirstRuneWithStrings(string(r))
}

func Range(start, end rune) Type {
	if end < start {
		end = start
	}
	return &rangeType{
		start: start,
		end:   end,
	}
}

func (r *rangeType) validate(ctx *parseContext, in string) <-chan Checked {
	out := make(chan Checked)
	go func() {
		defer close(out)
		if len(in) > 0 {
			first := rune(in[0])
			if first >= r.start && first <= r.end {
				out <- Checked{
					Suggestion: string(first),
					ToParse:    in[1:],
				}
			} else if r.suggestFn != nil {
				for _, checked := range r.suggestFn(in) {
					if checked != nil {
						out <- *checked
					}
				}
			}
		}

	}()
	return out
}

func (r *rangeType) WithSuggestionFunc(fn SuggestionFunc) Type {
	r.suggestFn = fn
	return r
}

type concat struct {
	type1 Type
	type2 Type
}

var _ Type = &concat{}

func Concat(types ...Type) Type {
	if len(types) == 0 {
		return (*concat)(nil)
	}
	if len(types) == 1 {
		return types[0]
	}
	var merged *concat
	for len(types) > 1 {
		merged = &concat{
			type1: types[len(types)-2],
			type2: types[len(types)-1],
		}
		types = append(types[:len(types)-2], merged)
	}
	return merged
}

func (c *concat) validate(ctx *parseContext, in string) <-chan Checked {
	out := make(chan Checked)
	go func() {
		defer close(out)
		if c != nil {
			checkedResults := []Checked{}
			for checked := range c.type1.validate(ctx.clone(), in) {
				checkedResults = append(checkedResults, checked)
			}
			for _, checkedResult := range checkedResults {
				for checked := range c.type2.validate(ctx.clone(), checkedResult.ToParse) {
					out <- Checked{
						Suggestion: checkedResult.Suggestion + checked.Suggestion,
						ToParse:    checked.ToParse,
					}
				}
			}
		} else {
			out <- Checked{
				Suggestion: "",
				ToParse:    in,
			}
		}
	}()
	return out
}

func (c *concat) WithSuggestionFunc(fn SuggestionFunc) Type {
	panic("not implemented")
}

type alternative struct {
	types     []Type
	suggestFn SuggestionFunc
}

var _ Type = &alternative{}

func Alternative(types ...Type) Type {
	return &alternative{
		types: types,
	}
}

func (a *alternative) validate(ctx *parseContext, in string) <-chan Checked {
	out := make(chan Checked)
	go func() {
		defer close(out)
		validated := false
		for _, t := range a.types {
			for checked := range t.validate(ctx.clone(), in) {
				out <- checked
				validated = true
			}
		}
		if a.suggestFn != nil && !validated {
			for _, checked := range a.suggestFn(in) {
				if checked != nil {
					out <- *checked
				}
			}
		}
	}()
	return out
}

func (a *alternative) WithSuggestionFunc(fn SuggestionFunc) Type {
	a.suggestFn = fn
	return a
}

var namedTypes = map[string]Type{}

type named struct {
	name string
}

var _ Type = &named{}

func Named(name string, t Type) Type {
	namedTypes[name] = t
	return t
}

const randomNamePrefix = "___random_name_"

func GetRandomName() string {
	suffixBigNum, err := rand.Int(rand.Reader, big.NewInt(10000000000))
	if err != nil {
		panic(err)
	}
	suffixNum := suffixBigNum.Int64()
	for i := range 50000 {
		name := fmt.Sprintf("%s%d", randomNamePrefix, i+int(suffixNum))
		if _, ok := namedTypes[name]; !ok {
			return name
		}
	}
	panic("cannot get random name")
}

func RefNamed(name string) *named {
	return &named{
		name: name,
	}
}

func (n *named) validate(ctx *parseContext, in string) <-chan Checked {
	np := namedPathNode{
		ruleName: n.name,
		input:    in,
	}

	if ctx.hasPathNode(np) {
		return closedCheckedChan
	}
	ctx.appendPathNode(np)
	if t, ok := namedTypes[n.name]; ok {
		return t.validate(ctx.clone(), in)
	}
	return closedCheckedChan
}

func (n *named) WithSuggestionFunc(fn SuggestionFunc) Type {
	panic("not implemented")
}

const SeqInf = -1

func Seq(min, max int, t Type) Type {
	if min < 0 {
		min = 0
	}
	if max < 0 && max != SeqInf {
		max = SeqInf
	}
	if max != SeqInf && max < min {
		max = min
	}
	if min == max {
		if min == 1 {
			return t
		}
		return Concat(slices.Repeat[[]Type]([]Type{t}, min)...)
	}
	if max == SeqInf {
		prefix := Seq(min, min, t)
		ruleName := GetRandomName()
		suffix := Named(ruleName,
			Alternative(
				Concat(),
				t,
				Concat(t,
					RefNamed(ruleName)),
			))
		return Concat(prefix, suffix)
	}
	types := make([]Type, 0, max-min+1)
	for i := min; i <= max; i++ {
		var rType Type = Concat(slices.Repeat[[]Type]([]Type{t}, i)...)
		types = append(types, rType)
	}
	return Alternative(types...)
}

func Opt(t Type) Type {
	return Seq(0, 1, t)
}
