package parsan_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/SAP/crossplane-provider-cloudfoundry/exporttool/parsan"
)

var _ = Describe("Rfc1035", func() {
	Describe("RFC1035Label", func() {
		It("sanitizes ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.RFC1035Label(nil))).To(Equal([]string{"x"}))
		})
		It("matches 'a'", func() {
			Expect(parsan.ParseAndSanitize("a", parsan.RFC1035Label(nil))).To(Equal([]string{"a"}))
		})
		It("matches 'kubernetes'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes", parsan.RFC1035Label(nil))).To(Equal([]string{"kubernetes"}))
		})
		It("matches 'kubernetes1'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes1", parsan.RFC1035Label(nil))).To(Equal([]string{"kubernetes1"}))
		})
		It("matches 'kubernetes-custom-resource'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes-custom-resource", parsan.RFC1035Label(nil))).To(Equal([]string{"kubernetes-custom-resource"}))
		})
		It("sanitizes 'kubernetes custom resource'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes custom resource", parsan.RFC1035Label(parsan.SuggestConstRune('-')))).To(Equal([]string{"kubernetes-custom-resource"}))
		})
		It("sanitizes '1kubernetes-custom-resource'", func() {
			Expect(parsan.ParseAndSanitize("1kubernetes-custom-resource", parsan.RFC1035Label(nil))).To(Equal([]string{"x1kubernetes-custom-resource", "xkubernetes-custom-resource"}))
		})
		It("sanitizes '1!kubernetes-custom-resource'", func() {
			Expect(parsan.ParseAndSanitize("1!kubernetes-custom-resource", parsan.RFC1035Label(parsan.SuggestConstRune('-')))).To(Equal([]string{"x1-kubernetes-custom-resource", "x-kubernetes-custom-resource"}))
		})
		It("sanitizes 'kubernetes-custom-resource!'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes-custom-resource!", parsan.RFC1035Label(nil))).To(Equal([]string{"kubernetes-custom-resourcex"}))
		})
		It("sanitizes '$@b@$rnetes-cu@$$!stom-reso@$urce!'", func() {
			Expect(parsan.ParseAndSanitize("$@b@$rnetes-cu@$$!stom-reso@$urce!", parsan.RFC1035Label(parsan.SuggestConstRune('-')))).To(Equal([]string{"x--b--rnetes-cu----stom-reso--urcex", "x-b--rnetes-cu----stom-reso--urcex"}))
		})
	})
	Describe("RFC1035Subdomain", func() {
		It("sanitizes ''", func() {
			Expect(parsan.ParseAndSanitize("", parsan.RFC1035Subdomain)).To(Equal([]string{"x"}))
		})
		It("matches 'kubernetes-custom-resource'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes-custom-resource", parsan.RFC1035Subdomain)).To(Equal([]string{"kubernetes-custom-resource"}))
		})
		It("sanitizes '1kubernetes-custom-resource'", func() {
			Expect(parsan.ParseAndSanitize("1kubernetes-custom-resource", parsan.RFC1035Subdomain)).To(Equal([]string{"x1kubernetes-custom-resource", "xkubernetes-custom-resource"}))
		})
		It("matches 'kubernetes-custom-resource1'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes-custom-resource1", parsan.RFC1035Subdomain)).To(Equal([]string{"kubernetes-custom-resource1"}))
		})
		It("matches 'a.b'", func() {
			Expect(parsan.ParseAndSanitize("a.b", parsan.RFC1035Subdomain)).To(Equal([]string{"a.b"}))
		})
		It("matches 'a.b.c'", func() {
			Expect(parsan.ParseAndSanitize("a.b.c", parsan.RFC1035Subdomain)).To(Equal([]string{"a.b.c"}))
		})
		It("matches 'kubernetes.custom.resource'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes.custom.resource", parsan.RFC1035Subdomain)).To(Equal([]string{"kubernetes.custom.resource"}))
		})
		It("matches 'kubernetes1.custom2.resource3'", func() {
			Expect(parsan.ParseAndSanitize("kubernetes1.custom2.resource3", parsan.RFC1035Subdomain)).To(Equal([]string{"kubernetes1.custom2.resource3"}))
		})
		It("sanitizes '1kubernetes.2custom.1resource'", func() {
			Expect(parsan.ParseAndSanitize("1kubernetes.2custom.3resource", parsan.RFC1035Subdomain)).To(Equal([]string{
				"x1kubernetes.x2custom.x3resource",
				"x1kubernetes.x2custom.xresource",
				"x1kubernetes.xcustom.x3resource",
				"xkubernetes.x2custom.x3resource",
				"x1kubernetes.xcustom.xresource",
				"xkubernetes.x2custom.xresource",
				"xkubernetes.xcustom.x3resource",
				"xkubernetes.xcustom.xresource",
			}))
		})
		It("sanitizes 'do+it+now'", func() {
			Expect(parsan.ParseAndSanitize("do+it+now", parsan.RFC1035Subdomain)).To(Equal([]string{"do-it-now"}))
		})
		It("sanitizes 'do+it+now.r!ght.n0w$'", func() {
			Expect(parsan.ParseAndSanitize("do+it+now.r!ght.n0w$", parsan.RFC1035Subdomain)).To(Equal([]string{"do-it-now.r-ght.n0wx"}))
		})
		It("matches 'a23456789012345678901234567890123456789012345678901234567890123'", func() {
			Expect(parsan.ParseAndSanitize("a23456789012345678901234567890123456789012345678901234567890123", parsan.RFC1035Subdomain)).To(Equal([]string{"a23456789012345678901234567890123456789012345678901234567890123"}))
		})
		It("sanitizes 'a234567890123456789012345678901234567890123456789012345678901234'", func() {
			Expect(parsan.ParseAndSanitize("a234567890123456789012345678901234567890123456789012345678901234", parsan.RFC1035Subdomain)).To(Equal([]string{"a23456789012345678901234567890123456789012345678901234567890123"}))
		})
		It("sanitizes '123456789012345678901234567890123456789012345678901234567890123'", func() {
			Expect(parsan.ParseAndSanitize("123456789012345678901234567890123456789012345678901234567890123", parsan.RFC1035Subdomain)).To(Equal([]string{"x23456789012345678901234567890123456789012345678901234567890123"}))
		})
		It("sanitizes '1234567890123456789012345678901234567890123456789012345678901234'", func() {
			Expect(parsan.ParseAndSanitize("1234567890123456789012345678901234567890123456789012345678901234", parsan.RFC1035Subdomain)).To(Equal([]string{"x23456789012345678901234567890123456789012345678901234567890123"}))
		})
	})
})
