package parsan_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/parsan"
)

var _ = Describe("Testing Type", func() {
	Describe("Terminal", func() {
		var t parsan.Type
		Context("with value 'a'", func() {
			BeforeEach(func() {
				t = parsan.Terminal("a")
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", t)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", t)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", t)).To(BeEmpty())
			})
		})
		Context("with value 'a1b'", func() {
			BeforeEach(func() {
				t = parsan.Terminal("a1b")
			})
			It("can parse 'a1b'", func() {
				Expect(parsan.ParseAndSanitize("a1b", t)).To(Equal([]string{"a1b"}))
			})
			It("cannot parse 'a1b1'", func() {
				Expect(parsan.ParseAndSanitize("a1b1", t)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", t)).To(BeEmpty())
			})
		})
	})
	Describe("Range", func() {
		var r parsan.Type
		Context("with value a-z (a)", func() {
			BeforeEach(func() {
				r = parsan.Range('a', 'z').WithSuggestionFunc(parsan.SuggestConstRune('a'))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", r)).To(Equal([]string{"a"}))
			})
			It("can parse 'z'", func() {
				Expect(parsan.ParseAndSanitize("z", r)).To(Equal([]string{"z"}))
			})
			It("can parse 'd'", func() {
				Expect(parsan.ParseAndSanitize("d", r)).To(Equal([]string{"d"}))
			})
			It("can parse 'A', suggesting 'a'", func() {
				Expect(parsan.ParseAndSanitize("A", r)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", r)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", r)).To(BeEmpty())
			})
		})
	})
	Describe("Concat", func() {
		var c parsan.Type
		Context("with empty value", func() {
			BeforeEach(func() {
				c = parsan.Concat()
			})
			It("is a nil type", func() {
				Expect(c).To(BeNil())
			})
			It("cannot parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", c)).To(BeEmpty())
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", c)).To(BeEmpty())
			})
			It("can parse ''", func() {
				Expect(parsan.ParseAndSanitize("", c)).To(Equal([]string{""}))
			})
		})
		Context("with a single Terminal ('a') value", func() {
			BeforeEach(func() {
				c = parsan.Concat(parsan.Terminal("a"))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", c)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", c)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", c)).To(BeEmpty())
			})
		})
		Context("with a Terminal('a'), Terminal('b') value", func() {
			BeforeEach(func() {
				c = parsan.Concat(parsan.Terminal("a"), parsan.Terminal("b"))
			})
			It("can parse 'ab'", func() {
				Expect(parsan.ParseAndSanitize("ab", c)).To(Equal([]string{"ab"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", c)).To(BeEmpty())
			})
			It("cannot parse 'aba'", func() {
				Expect(parsan.ParseAndSanitize("aba", c)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", c)).To(BeEmpty())
			})
		})
		Context("with a Terminal('a'), Terminal('b'), Terminal('c') value", func() {
			BeforeEach(func() {
				c = parsan.Concat(
					parsan.Terminal("a"),
					parsan.Terminal("b"),
					parsan.Terminal("c"),
				)
			})
			It("can parse 'abc'", func() {
				Expect(parsan.ParseAndSanitize("abc", c)).To(Equal([]string{"abc"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", c)).To(BeEmpty())
			})
			It("cannot parse 'aba'", func() {
				Expect(parsan.ParseAndSanitize("aba", c)).To(BeEmpty())
			})
			It("cannot parse 'abca'", func() {
				Expect(parsan.ParseAndSanitize("abca", c)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", c)).To(BeEmpty())
			})
		})
		Context("with a Terminal('a'), Range('A', 'Z', 'X'), Terminal('c') value", func() {
			BeforeEach(func() {
				c = parsan.Concat(
					parsan.Terminal("a"),
					parsan.Range('A', 'Z').WithSuggestionFunc(parsan.SuggestConstRune('X')),
					parsan.Terminal("c"),
				)
			})
			It("can parse 'aBc'", func() {
				Expect(parsan.ParseAndSanitize("aBc", c)).To(Equal([]string{"aBc"}))
			})
			It("can parse 'abc'", func() {
				Expect(parsan.ParseAndSanitize("aXc", c)).To(Equal([]string{"aXc"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", c)).To(BeEmpty())
			})
			It("cannot parse 'aba'", func() {
				Expect(parsan.ParseAndSanitize("aba", c)).To(BeEmpty())
			})
			It("cannot parse 'abca'", func() {
				Expect(parsan.ParseAndSanitize("abca", c)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", c)).To(BeEmpty())
			})
		})
	})
	Describe("Alternative", func() {
		var a parsan.Type
		Context("with empty value", func() {
			BeforeEach(func() {
				a = parsan.Alternative()
			})
			It("cannot parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", a)).To(BeEmpty())
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", a)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", a)).To(BeEmpty())
			})

		})
		Context("with a single Terminal ('a') value", func() {
			BeforeEach(func() {
				a = parsan.Alternative(parsan.Terminal("a"))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", a)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", a)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", a)).To(BeEmpty())
			})
		})
		Context("with a Terminal('a')/Terminal('b') value", func() {
			BeforeEach(func() {
				a = parsan.Alternative(
					parsan.Terminal("a"),
					parsan.Terminal("b"),
				)
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", a)).To(Equal([]string{"a"}))
			})
			It("can parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", a)).To(Equal([]string{"b"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", a)).To(BeEmpty())
			})
			It("cannot parse 'ba'", func() {
				Expect(parsan.ParseAndSanitize("ba", a)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", a)).To(BeEmpty())
			})
		})
	})
	Describe("Named", func() {
		var n parsan.Type
		Context("named Terminal('a')", func() {
			BeforeEach(func() {
				n = parsan.Named("term-a", parsan.Terminal("a"))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", n)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", n)).To(BeEmpty())
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", n)).To(BeEmpty())
			})
			It("can parse 'a' when referring to its name", func() {
				Expect(parsan.ParseAndSanitize("a", parsan.RefNamed("term-a"))).To(Equal([]string{"a"}))
			})
			It("cannot parse 'a' when referring to a nonexisting name", func() {
				Expect(parsan.ParseAndSanitize("a", parsan.RefNamed("nonexisting"))).To(BeEmpty())
			})
			It("cannot parse 'b' when referring to its name", func() {
				Expect(parsan.ParseAndSanitize("b", parsan.RefNamed("term-b"))).To(BeEmpty())
			})
		})
		Context("rec = Terminal('a') | Terminal('a') rec", func() {
			BeforeEach(func() {
				n = parsan.Named("term-a",
					parsan.Alternative(
						parsan.Terminal("a"),
						parsan.Concat(
							parsan.Terminal("a"),
							parsan.RefNamed("term-a"),
						),
					))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", n)).To(Equal([]string{"a"}))
			})
			It("can parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", n)).To(Equal([]string{"aa"}))
			})
			It("can parse 'aaa'", func() {
				Expect(parsan.ParseAndSanitize("aaa", n)).To(Equal([]string{"aaa"}))
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", n)).To(BeEmpty())
			})
			It("cannot parse 'ab'", func() {
				Expect(parsan.ParseAndSanitize("ab", n)).To(BeEmpty())
			})
			It("cannot parse 'aaaaaab'", func() {
				Expect(parsan.ParseAndSanitize("aaaaaab", n)).To(BeEmpty())
			})
		})
	})
	Describe("Seq", func() {
		var s parsan.Type
		Context("seq(0,0, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(0, 0, parsan.Terminal("a"))
			})
			It("can parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(Equal([]string{""}))
			})
			It("cannot parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(BeEmpty())
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
			It("cannot parse 'aaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaa", s)).To(BeEmpty())
			})
		})
		Context("seq(1,1, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(1, 1, parsan.Terminal("a"))
			})
			It("it cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(BeEmpty())
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
			It("cannot parse 'aaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaa", s)).To(BeEmpty())
			})
		})
		Context("seq(2,2, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(2, 2, parsan.Terminal("a"))
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(BeEmpty())
			})
			It("cannot parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(BeEmpty())
			})
			It("can parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
			It("cannot parse 'aaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaa", s)).To(BeEmpty())
			})
		})
		Context("seq(0,1, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(0, 1, parsan.Terminal("a"))
			})
			It("can parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(Equal([]string{""}))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{"a"}))
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", s)).To(BeEmpty())
			})
			It("cannot parse 'aaa'", func() {
				Expect(parsan.ParseAndSanitize("aaa", s)).To(BeEmpty())
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
		})
		Context("seq(1,3, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(1, 3, parsan.Terminal("a"))
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(BeEmpty())
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{"a"}))
			})
			It("can parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
			})
			It("can parse 'aaa'", func() {
				Expect(parsan.ParseAndSanitize("aaa", s)).To(Equal([]string{"aaa"}))
			})
			It("cannot parse 'aaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaa", s)).To(BeEmpty())
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
		})
		Context("seq(0, inf, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(0, parsan.SeqInf, parsan.Terminal("a"))
			})
			It("can parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(Equal([]string{""}))
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{"a"}))
			})
			It("can parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
			})
			It("can parse 'aaaaaaaaaaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaaaaaaaaaa", s)).To(Equal([]string{"aaaaaaaaaaaa"}))
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
			It("cannot parse 'ab'", func() {
				Expect(parsan.ParseAndSanitize("ab", s)).To(BeEmpty())
			})
		})
		Context("seq(1, inf, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(1, parsan.SeqInf, parsan.Terminal("a"))
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(BeEmpty())
			})
			It("can parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{"a"}))
			})
			It("can parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
			})
			It("can parse 'aaaaaaaaaaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaaaaaaaaaa", s)).To(Equal([]string{"aaaaaaaaaaaa"}))
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
			It("cannot parse 'ab'", func() {
				Expect(parsan.ParseAndSanitize("ab", s)).To(BeEmpty())
			})
		})
		Context("seq(3, inf, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(3, parsan.SeqInf, parsan.Terminal("a"))
			})
			It("cannot parse ''", func() {
				Expect(parsan.ParseAndSanitize("", s)).To(BeEmpty())
			})
			It("cannot parse 'a'", func() {
				Expect(parsan.ParseAndSanitize("a", s)).To(BeEmpty())
			})
			It("cannot parse 'aa'", func() {
				Expect(parsan.ParseAndSanitize("aa", s)).To(BeEmpty())
			})
			It("can parse 'aaa'", func() {
				Expect(parsan.ParseAndSanitize("aaa", s)).To(Equal([]string{"aaa"}))
			})
			It("can parse 'aaaaaaaaaaaa'", func() {
				Expect(parsan.ParseAndSanitize("aaaaaaaaaaaa", s)).To(Equal([]string{"aaaaaaaaaaaa"}))
			})
			It("cannot parse 'b'", func() {
				Expect(parsan.ParseAndSanitize("b", s)).To(BeEmpty())
			})
			It("cannot parse 'ab'", func() {
				Expect(parsan.ParseAndSanitize("ab", s)).To(BeEmpty())
			})
		})
	})
})
