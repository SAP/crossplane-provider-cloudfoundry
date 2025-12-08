package parsan_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
)

var _ = Describe("Testing Type", func() {
	Describe("Terminal", func() {
		var t parsan.Rule
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
		Context("with valid maxLength", func() {
			It("can be parsed", func() {
				t = parsan.Terminal("abcd").WithMaxLength(4)
				Expect(parsan.ParseAndSanitize("abcd", t)).To(Equal([]string{"abcd"}))
			})
		})
		Context("with invalid maxLength", func() {
			It("panics", func() {
				Expect(func() {
					parsan.Terminal("abcd").WithMaxLength(3)
				}).To(Panic())
				Expect(func() {
					parsan.Terminal("abcd").WithMaxLength(5)
				}).To(Panic())
			})
		})
	})
	Describe("Range", func() {
		var r parsan.Rule
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
		Context("with valid maxLength", func() {
			It("can be parsed", func() {
				r = parsan.Range('a', 'z').WithMaxLength(1)
				Expect(parsan.ParseAndSanitize("c", r)).To(Equal([]string{"c"}))
			})
		})
		Context("with invalid maxLength", func() {
			It("panics", func() {
				Expect(func() {
					parsan.Range('a', 'z').WithMaxLength(0)
				}).To(Panic())
				Expect(func() {
					parsan.Range('a', 'z').WithMaxLength(2)
				}).To(Panic())
			})
		})
	})
	Describe("Concat", func() {
		var c parsan.Rule
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
		Context("with valid MaxLength", func() {
			BeforeEach(func() {
				c = parsan.Concat(
					parsan.Terminal("ab"),
					parsan.Terminal("cd"),
					parsan.Terminal("ef"),
				)
			})
			It("can parse if MaxLength is large enough", func() {
				c.WithMaxLength(6)
				Expect(parsan.ParseAndSanitize("abcdef", c)).To(Equal([]string{"abcdef"}))
			})
			It("can parse if MaxLength is larger than enough", func() {
				c.WithMaxLength(7)
				Expect(parsan.ParseAndSanitize("abcdef", c)).To(Equal([]string{"abcdef"}))
			})
			It("cannot parse if MaxLength is too low", func() {
				c.WithMaxLength(5)
				Expect(parsan.ParseAndSanitize("abcdef", c)).To(BeEmpty())
			})
		})
	})
	Describe("Alternative", func() {
		var a parsan.Rule
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
		Context("a / ab / abc", func() {
			BeforeEach(func() {
				a = parsan.Alternative(
					parsan.Terminal("a"),
					parsan.Terminal("ab"),
					parsan.Terminal("abc"),
				)
			})
			Context("with MaxLength = 4", func() {
				It("validates 'abc'", func() {
					Expect(parsan.ParseAndSanitize("abc",
						a.WithMaxLength(4))).To(Equal([]string{"abc"}))
				})
			})
			Context("with MaxLength = 3", func() {
				It("validates 'abc'", func() {
					Expect(parsan.ParseAndSanitize("abc",
						a.WithMaxLength(3))).To(Equal([]string{"abc"}))
				})
			})
			Context("with MaxLength = 2", func() {
				It("cannot parse", func() {
					Expect(parsan.ParseAndSanitize("abc",
						a.WithMaxLength(2))).To(Equal([]string{"ab"}))
				})
			})
		})
	})
	Describe("Named", func() {
		var n parsan.Rule
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
				Expect(parsan.ParseAndSanitize("a", parsan.Ref("term-a"))).To(Equal([]string{"a"}))
			})
			It("cannot parse 'a' when referring to a nonexisting name", func() {
				Expect(parsan.ParseAndSanitize("a", parsan.Ref("nonexisting"))).To(BeEmpty())
			})
			It("cannot parse 'b' when referring to its name", func() {
				Expect(parsan.ParseAndSanitize("b", parsan.Ref("term-b"))).To(BeEmpty())
			})
		})
		Context("rec = Terminal('a') | Terminal('a') rec", func() {
			BeforeEach(func() {
				n = parsan.Named("term-a",
					parsan.Alternative(
						parsan.Terminal("a"),
						parsan.Concat(
							parsan.Terminal("a"),
							parsan.Ref("term-a"),
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
		var s parsan.Rule
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
			Context("With MaxLength = 0", func() {
				It("can parse ''", func() {
					Expect(parsan.ParseAndSanitize("", s.WithMaxLength(0))).To(Equal([]string{""}))
				})
			})
			Context("With MaxLength = 1", func() {
				It("panics", func() {
					Expect(func() {
						s.WithMaxLength(1)
					}).To(Panic())
				})
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
			Context("With MaxLength = 1", func() {
				It("can parse 'a'", func() {
					Expect(parsan.ParseAndSanitize("a", s.WithMaxLength(1))).To(Equal([]string{"a"}))
				})
			})
			Context("With MaxLength = 2", func() {
				It("panics", func() {
					Expect(func() {
						s.WithMaxLength(2)
					}).To(Panic())
				})
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
			Context("With MaxLength = 1", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(1)
				})
				It("cannot parse 'a'", func() {
					Expect(parsan.ParseAndSanitize("a", s)).To(BeEmpty())
				})
				It("cannot parse 'aa'", func() {
					Expect(parsan.ParseAndSanitize("a", s)).To(BeEmpty())
				})
			})
			Context("With MaxLength = 2", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(2)
				})
				It("cannot parse 'a'", func() {
					Expect(parsan.ParseAndSanitize("a", s)).To(BeEmpty())
				})
				It("can parse 'aa'", func() {
					Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
				})
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
			Context("With MaxLength = 0", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(0)
				})
				It("can parse ''", func() {
					Expect(parsan.ParseAndSanitize("", s)).To(Equal([]string{""}))
				})
				It("can parse 'a'", func() {
					Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{""}))
				})
			})
			Context("With MaxLength = 1", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(1)
				})
				It("can parse ''", func() {
					Expect(parsan.ParseAndSanitize("", s)).To(Equal([]string{""}))
				})
				It("cannot parse 'a'", func() {
					Expect(parsan.ParseAndSanitize("a", s)).To(Equal([]string{"a"}))
				})
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
			Context("With MaxLength = 2", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(2)
				})
				It("can parse 'aa'", func() {
					Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
				})
				It("can parse 'aaa'", func() {
					Expect(parsan.ParseAndSanitize("aaa", s)).To(Equal([]string{"aa"}))
				})
			})
			Context("With MaxLength = 3", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(3)
				})
				It("can parse 'aa'", func() {
					Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
				})
				It("can parse 'aaa'", func() {
					Expect(parsan.ParseAndSanitize("aaa", s)).To(Equal([]string{"aaa"}))
				})
			})
		})
		Context("seq(0, inf, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(0, parsan.Unlimited, parsan.Terminal("a"))
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
			Context("With MaxLength = 2", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(2)
				})
				It("can parse 'aa'", func() {
					Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
				})
				It("can parse 'aaa'", func() {
					Expect(parsan.ParseAndSanitize("aaa", s)).To(Equal([]string{"aa"}))
				})
			})
			Context("With MaxLength = 3", func() {
				BeforeEach(func() {
					s = s.WithMaxLength(3)
				})
				It("can parse 'aa'", func() {
					Expect(parsan.ParseAndSanitize("aa", s)).To(Equal([]string{"aa"}))
				})
				It("can parse 'aaa'", func() {
					Expect(parsan.ParseAndSanitize("aaa", s)).To(Equal([]string{"aaa"}))
				})
				It("can parse 'aaaa'", func() {
					Expect(parsan.ParseAndSanitize("aaaa", s)).To(Equal([]string{"aaa"}))
				})
			})
		})
		Context("seq(1, inf, Terminal(a))", func() {
			BeforeEach(func() {
				s = parsan.Seq(1, parsan.Unlimited, parsan.Terminal("a"))
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
				s = parsan.Seq(3, parsan.Unlimited, parsan.Terminal("a"))
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
