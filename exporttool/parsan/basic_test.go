package parsan_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
)

var _ = Describe("Basic", func() {
	Describe("Digit", func() {
		It("doesn't match ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.Digit(nil))).To(BeEmpty())
		})
		It("doesn't match '00'", func() {
			Expect(parsan.ParseAndSanitize("00", parsan.Digit(nil))).To(BeEmpty())
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.Digit(nil))).To(Equal([]string{"9"}))
		})
		It("matches '0'", func() {
			Expect(parsan.ParseAndSanitize("0", parsan.Digit(nil))).To(Equal([]string{"0"}))
		})
		It("matches '2'", func() {
			Expect(parsan.ParseAndSanitize("2", parsan.Digit(nil))).To(Equal([]string{"2"}))
		})
		It("matches '7'", func() {
			Expect(parsan.ParseAndSanitize("7", parsan.Digit(nil))).To(Equal([]string{"7"}))
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.Digit(nil))).To(Equal([]string{"9"}))
		})
		It("doesn't match 'a'", func() {
			Expect(parsan.ParseAndSanitize("a", parsan.Digit(nil))).To(BeEmpty())
		})
	})
	Describe("Letter", func() {
		It("doesn't match ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.Letter(nil))).To(BeEmpty())
		})
		It("doesn't match 'aA'", func() {
			Expect(parsan.ParseAndSanitize("aA", parsan.Letter(nil))).To(BeEmpty())
		})
		It("matches 'a'", func() {
			Expect(parsan.ParseAndSanitize("a", parsan.Letter(nil))).To(Equal([]string{"a"}))
		})
		It("matches 'c'", func() {
			Expect(parsan.ParseAndSanitize("c", parsan.Letter(nil))).To(Equal([]string{"c"}))
		})
		It("matches 'z'", func() {
			Expect(parsan.ParseAndSanitize("z", parsan.Letter(nil))).To(Equal([]string{"z"}))
		})
		It("matches 'A'", func() {
			Expect(parsan.ParseAndSanitize("A", parsan.Letter(nil))).To(Equal([]string{"A"}))
		})
		It("matches 'F'", func() {
			Expect(parsan.ParseAndSanitize("F", parsan.Letter(nil))).To(Equal([]string{"F"}))
		})
		It("matches 'Z'", func() {
			Expect(parsan.ParseAndSanitize("Z", parsan.Letter(nil))).To(Equal([]string{"Z"}))
		})
		It("doesn't match '1'", func() {
			Expect(parsan.ParseAndSanitize("1", parsan.Letter(nil))).To(BeEmpty())
		})
	})
	Describe("LetDig", func() {
		It("doesn't match ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.LetDig(nil))).To(BeEmpty())
		})
		It("doesn't match 'aA'", func() {
			Expect(parsan.ParseAndSanitize("aA", parsan.LetDig(nil))).To(BeEmpty())
		})
		It("matches 'a'", func() {
			Expect(parsan.ParseAndSanitize("a", parsan.LetDig(nil))).To(Equal([]string{"a"}))
		})
		It("matches 'c'", func() {
			Expect(parsan.ParseAndSanitize("c", parsan.LetDig(nil))).To(Equal([]string{"c"}))
		})
		It("matches 'z'", func() {
			Expect(parsan.ParseAndSanitize("z", parsan.LetDig(nil))).To(Equal([]string{"z"}))
		})
		It("matches 'A'", func() {
			Expect(parsan.ParseAndSanitize("A", parsan.LetDig(nil))).To(Equal([]string{"A"}))
		})
		It("matches 'F'", func() {
			Expect(parsan.ParseAndSanitize("F", parsan.LetDig(nil))).To(Equal([]string{"F"}))
		})
		It("matches 'Z'", func() {
			Expect(parsan.ParseAndSanitize("Z", parsan.LetDig(nil))).To(Equal([]string{"Z"}))
		})
		It("doesn't match '00'", func() {
			Expect(parsan.ParseAndSanitize("00", parsan.LetDig(nil))).To(BeEmpty())
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.LetDig(nil))).To(Equal([]string{"9"}))
		})
		It("matches '0'", func() {
			Expect(parsan.ParseAndSanitize("0", parsan.LetDig(nil))).To(Equal([]string{"0"}))
		})
		It("matches '2'", func() {
			Expect(parsan.ParseAndSanitize("2", parsan.LetDig(nil))).To(Equal([]string{"2"}))
		})
		It("matches '7'", func() {
			Expect(parsan.ParseAndSanitize("7", parsan.LetDig(nil))).To(Equal([]string{"7"}))
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.LetDig(nil))).To(Equal([]string{"9"}))
		})
		It("doesn't match '-'", func() {
			Expect(parsan.ParseAndSanitize("-", parsan.LetDig(nil))).To(BeEmpty())
		})
	})
	Describe("LetDigHyp", func() {
		It("doesn't match ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.LetDigHyp(nil))).To(BeEmpty())
		})
		It("doesn't match 'aA'", func() {
			Expect(parsan.ParseAndSanitize("aA", parsan.LetDigHyp(nil))).To(BeEmpty())
		})
		It("matches 'a'", func() {
			Expect(parsan.ParseAndSanitize("a", parsan.LetDigHyp(nil))).To(Equal([]string{"a"}))
		})
		It("matches 'c'", func() {
			Expect(parsan.ParseAndSanitize("c", parsan.LetDigHyp(nil))).To(Equal([]string{"c"}))
		})
		It("matches 'z'", func() {
			Expect(parsan.ParseAndSanitize("z", parsan.LetDigHyp(nil))).To(Equal([]string{"z"}))
		})
		It("matches 'A'", func() {
			Expect(parsan.ParseAndSanitize("A", parsan.LetDigHyp(nil))).To(Equal([]string{"A"}))
		})
		It("matches 'F'", func() {
			Expect(parsan.ParseAndSanitize("F", parsan.LetDigHyp(nil))).To(Equal([]string{"F"}))
		})
		It("matches 'Z'", func() {
			Expect(parsan.ParseAndSanitize("Z", parsan.LetDigHyp(nil))).To(Equal([]string{"Z"}))
		})
		It("doesn't match '00'", func() {
			Expect(parsan.ParseAndSanitize("00", parsan.LetDigHyp(nil))).To(BeEmpty())
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.LetDigHyp(nil))).To(Equal([]string{"9"}))
		})
		It("matches '0'", func() {
			Expect(parsan.ParseAndSanitize("0", parsan.LetDigHyp(nil))).To(Equal([]string{"0"}))
		})
		It("matches '2'", func() {
			Expect(parsan.ParseAndSanitize("2", parsan.LetDigHyp(nil))).To(Equal([]string{"2"}))
		})
		It("matches '7'", func() {
			Expect(parsan.ParseAndSanitize("7", parsan.LetDigHyp(nil))).To(Equal([]string{"7"}))
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.LetDigHyp(nil))).To(Equal([]string{"9"}))
		})
		It("matches '-'", func() {
			Expect(parsan.ParseAndSanitize("-", parsan.LetDigHyp(nil))).To(Equal([]string{"-"}))
		})
		It("suggests for '!'", func() {
			Expect(parsan.ParseAndSanitize("!", parsan.LetDigHyp(parsan.SuggestConstRune('-')))).To(Equal([]string{"-"}))
		})
	})
	Describe("LDHStr", func() {
		It("doesn't match ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.LDHStr(nil))).To(BeEmpty())
		})
		It("matches 'a'", func() {
			Expect(parsan.ParseAndSanitize("a", parsan.LDHStr(nil))).To(Equal([]string{"a"}))
		})
		It("matches 'c'", func() {
			Expect(parsan.ParseAndSanitize("c", parsan.LDHStr(nil))).To(Equal([]string{"c"}))
		})
		It("matches 'z'", func() {
			Expect(parsan.ParseAndSanitize("z", parsan.LDHStr(nil))).To(Equal([]string{"z"}))
		})
		It("matches 'A'", func() {
			Expect(parsan.ParseAndSanitize("A", parsan.LDHStr(nil))).To(Equal([]string{"A"}))
		})
		It("matches 'F'", func() {
			Expect(parsan.ParseAndSanitize("F", parsan.LDHStr(nil))).To(Equal([]string{"F"}))
		})
		It("matches 'Z'", func() {
			Expect(parsan.ParseAndSanitize("Z", parsan.LDHStr(nil))).To(Equal([]string{"Z"}))
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.LDHStr(nil))).To(Equal([]string{"9"}))
		})
		It("matches '0'", func() {
			Expect(parsan.ParseAndSanitize("0", parsan.LDHStr(nil))).To(Equal([]string{"0"}))
		})
		It("matches '2'", func() {
			Expect(parsan.ParseAndSanitize("2", parsan.LDHStr(nil))).To(Equal([]string{"2"}))
		})
		It("matches '7'", func() {
			Expect(parsan.ParseAndSanitize("7", parsan.LDHStr(nil))).To(Equal([]string{"7"}))
		})
		It("matches '9'", func() {
			Expect(parsan.ParseAndSanitize("9", parsan.LDHStr(nil))).To(Equal([]string{"9"}))
		})
		It("matches '-'", func() {
			Expect(parsan.ParseAndSanitize("-", parsan.LDHStr(nil))).To(Equal([]string{"-"}))
		})
		It("suggests for '!'", func() {
			Expect(parsan.ParseAndSanitize("!", parsan.LDHStr(parsan.SuggestConstRune('-')))).To(Equal([]string{"-"}))
		})
		It("matches 'alpha'", func() {
			Expect(parsan.ParseAndSanitize("alpha", parsan.LDHStr(nil))).To(Equal([]string{"alpha"}))
		})
		It("matches '1654232105'", func() {
			Expect(parsan.ParseAndSanitize("1654232105", parsan.LDHStr(nil))).To(Equal([]string{"1654232105"}))
		})
		It("matches 'alpha-beta'", func() {
			Expect(parsan.ParseAndSanitize("alpha-beta", parsan.LDHStr(nil))).To(Equal([]string{"alpha-beta"}))
		})
		It("matches 'alpha-beta-55'", func() {
			Expect(parsan.ParseAndSanitize("alpha-beta-55", parsan.LDHStr(nil))).To(Equal([]string{"alpha-beta-55"}))
		})
		It("matches '-13'", func() {
			Expect(parsan.ParseAndSanitize("-13", parsan.LDHStr(nil))).To(Equal([]string{"-13"}))
		})
		It("matches 'gamma-'", func() {
			Expect(parsan.ParseAndSanitize("gamma-", parsan.LDHStr(nil))).To(Equal([]string{"gamma-"}))
		})
		It("suggests-for 'do it now!'", func() {
			Expect(parsan.ParseAndSanitize("do it now!", parsan.LDHStr(parsan.SuggestConstRune('-')))).To(Equal([]string{"do-it-now-"}))
		})
	})
})
