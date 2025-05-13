package orgquota

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/utils/ptr"
)

// ptrString turns any pointer into a string. If the pointer is nil,
// the function returns the string "nil". Otherwise, it returns the
// string representation of the pointed to value using the '%v' format
// string.
func ptrString[T any](v *T) string {
	if v == nil {
		return "nil"
	}
	return fmt.Sprintf("%v", *v)
}

func TestIntpToFloatp(t *testing.T) {
	testValues := []struct {
		i *int
		f *float64
	}{
		{i: nil, f: nil},
		{i: ptr.To(1), f: ptr.To(1.0)},
		{i: ptr.To(0), f: ptr.To(0.0)},
		{i: ptr.To(-1), f: ptr.To(-1.0)},
		{i: ptr.To(1000), f: ptr.To(1000.0)},
	}

	for _, testValue := range testValues {
		t.Logf("testing intpToFloatp: i=%s\n", ptrString(testValue.i))
		if f := intpToFloatp(testValue.i); !ptr.Equal(f, testValue.f) {
			t.Errorf("invalid return value: %s, expected: %s", ptrString(f), ptrString(testValue.f))
		}

	}
}

func orgsString(orgs []*string) string {
	orgStrings := make([]string, len(orgs))
	for i := range orgs {
		orgStrings[i] = ptrString(orgs[i])
	}
	return strings.Join(orgStrings, ",")
}

func TestOrgsEqual(t *testing.T) {
	testValues := []struct {
		orgs1 []*string
		orgs2 []*string
		equal bool
	}{
		{
			orgs1: []*string{},
			orgs2: []*string{},
			equal: true,
		},
		{
			orgs1: []*string{nil, nil, nil},
			orgs2: []*string{},
			equal: true,
		},
		{
			orgs1: []*string{ptr.To("org1")},
			orgs2: []*string{ptr.To("org1")},
			equal: true,
		},
		{
			orgs1: []*string{ptr.To("org1"), ptr.To("org2")},
			orgs2: []*string{ptr.To("org1"), ptr.To("org2")},
			equal: true,
		},
		{
			orgs1: []*string{ptr.To("org2"), ptr.To("org1")},
			orgs2: []*string{ptr.To("org1"), ptr.To("org2")},
			equal: true,
		},
		{
			orgs1: []*string{ptr.To("org2"), nil, ptr.To("org1")},
			orgs2: []*string{ptr.To("org1"), ptr.To("org2")},
			equal: true,
		},
		{
			orgs1: []*string{ptr.To("org2")},
			orgs2: []*string{ptr.To("org1")},
			equal: false,
		},
		{
			orgs1: []*string{},
			orgs2: []*string{ptr.To("org2")},
			equal: false,
		},
		{
			orgs1: []*string{ptr.To("org1")},
			orgs2: []*string{},
			equal: false,
		},
	}

	for _, testValue := range testValues {
		t.Logf("testing orgsEqual, orgs1: %s - orgs2: %s",
			orgsString(testValue.orgs1),
			orgsString(testValue.orgs2),
		)
		if result := orgsEqual(testValue.orgs1, testValue.orgs2); result != testValue.equal {
			t.Errorf("orgsEqual failed - expected: %t, got: %t", testValue.equal, result)
		}
	}
}

func TestPtrCast(t *testing.T) {
	if ptr.Deref(ptrCast[float64, int](nil, 0.0), -1000) != 0 {
		t.Error("ptrCast[float64, int](nil, 0.0)) != 0")
	}
	if ptr.Deref(ptrCast[int, float64](nil, 0), -1000) != 0.0 {
		t.Error("ptrCast[int, float64](nil, 0) != 0.0")
	}
	if ptr.Deref(ptrCast[float64, int](ptr.To(15.0), 0.0), -1000) != 15 {
		t.Error("ptrCast[float64, int](15.0, 0.0)) != 15")
	}
	if ptr.Deref(ptrCast[int, float64](ptr.To(15), 0), -1000) != 15.0 {
		t.Error("ptrCast[int, float64](nil, 0) != 15.0")
	}
}

func TestPtrDef(t *testing.T) {
	if ptr.Deref(ptrDef(nil, -5), -1000) != -5 {
		t.Error("ptrDef(nil, -5) != -5")
	}
	if ptr.Deref(ptrDef(nil, -5.0), -1000.0) != -5.0 {
		t.Error("ptrDef(nil, -5.0) != -5.0")
	}
	if ptr.Deref(ptrDef(nil, true), false) != true {
		t.Error("ptrDef(nil, true) != true")
	}
	if ptr.Deref(ptrDef(ptr.To(10), -5), -1000) != 10 {
		t.Error("ptrDef(10, -5) != 10")
	}
	if ptr.Deref(ptrDef(ptr.To(10.0), -5.0), -1000.0) != 10.0 {
		t.Error("ptrDef(10.0, -5.0) != 10.0")
	}
	if ptr.Deref(ptrDef(ptr.To(false), true), true) != false {
		t.Error("ptrDef(false, true) != false")
	}
}
