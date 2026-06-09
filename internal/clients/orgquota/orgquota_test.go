package orgquota

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
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

func TestLateInitialize(t *testing.T) {
	fullResource := &resource.OrganizationQuota{}
	fullResource.GUID = "test-guid"
	fullResource.Name = "test-quota"
	fullResource.Services.PaidServicesAllowed = true
	fullResource.Services.TotalServiceInstances = ptr.To(5)
	fullResource.Services.TotalServiceKeys = ptr.To(10)
	fullResource.Apps.PerProcessMemoryInMB = ptr.To(1024)
	fullResource.Apps.TotalInstances = ptr.To(20)
	fullResource.Apps.LogRateLimitInBytesPerSecond = ptr.To(4096)
	fullResource.Apps.PerAppTasks = ptr.To(8)
	fullResource.Apps.TotalMemoryInMB = ptr.To(2048)
	fullResource.Routes.TotalRoutes = ptr.To(100)
	fullResource.Routes.TotalReservedPorts = ptr.To(5)
	fullResource.Domains.TotalDomains = ptr.To(3)
	fullResource.Relationships.Organizations.Data = []resource.Relationship{
		{GUID: "org-guid-1"},
		{GUID: "org-guid-2"},
	}

	t.Run("all fields nil - all populated", func(t *testing.T) {
		spec := &v1alpha1.OrgQuotaParameters{}
		changed := LateInitialize(spec, fullResource)
		if !changed {
			t.Error("expected changed=true when all fields nil")
		}
		if ptr.Deref(spec.Name, "") != "test-quota" {
			t.Errorf("Name: got %q, want %q", ptr.Deref(spec.Name, ""), "test-quota")
		}
		if ptr.Deref(spec.AllowPaidServicePlans, false) != true {
			t.Error("AllowPaidServicePlans not populated")
		}
		if ptr.Deref(spec.InstanceMemory, 0) != 1024 {
			t.Errorf("InstanceMemory: got %v, want 1024", ptr.Deref(spec.InstanceMemory, 0))
		}
		if ptr.Deref(spec.TotalAppInstances, 0) != 20 {
			t.Errorf("TotalAppInstances: got %v, want 20", ptr.Deref(spec.TotalAppInstances, 0))
		}
		if ptr.Deref(spec.TotalAppLogRateLimit, 0) != 4096 {
			t.Errorf("TotalAppLogRateLimit: got %v, want 4096", ptr.Deref(spec.TotalAppLogRateLimit, 0))
		}
		if ptr.Deref(spec.TotalAppTasks, 0) != 8 {
			t.Errorf("TotalAppTasks: got %v, want 8", ptr.Deref(spec.TotalAppTasks, 0))
		}
		if ptr.Deref(spec.TotalMemory, 0) != 2048 {
			t.Errorf("TotalMemory: got %v, want 2048", ptr.Deref(spec.TotalMemory, 0))
		}
		if ptr.Deref(spec.TotalPrivateDomains, 0) != 3 {
			t.Errorf("TotalPrivateDomains: got %v, want 3", ptr.Deref(spec.TotalPrivateDomains, 0))
		}
		if ptr.Deref(spec.TotalRoutePorts, 0) != 5 {
			t.Errorf("TotalRoutePorts: got %v, want 5", ptr.Deref(spec.TotalRoutePorts, 0))
		}
		if ptr.Deref(spec.TotalRoutes, 0) != 100 {
			t.Errorf("TotalRoutes: got %v, want 100", ptr.Deref(spec.TotalRoutes, 0))
		}
		if ptr.Deref(spec.TotalServiceKeys, 0) != 10 {
			t.Errorf("TotalServiceKeys: got %v, want 10", ptr.Deref(spec.TotalServiceKeys, 0))
		}
		if ptr.Deref(spec.TotalServices, 0) != 5 {
			t.Errorf("TotalServices: got %v, want 5", ptr.Deref(spec.TotalServices, 0))
		}
		if len(spec.Orgs) != 2 {
			t.Fatalf("Orgs length: got %d, want 2", len(spec.Orgs))
		}
		if ptr.Deref(spec.Orgs[0], "") != "org-guid-1" {
			t.Errorf("Orgs[0]: got %q, want %q", ptr.Deref(spec.Orgs[0], ""), "org-guid-1")
		}
		if ptr.Deref(spec.Orgs[1], "") != "org-guid-2" {
			t.Errorf("Orgs[1]: got %q, want %q", ptr.Deref(spec.Orgs[1], ""), "org-guid-2")
		}
	})

	t.Run("some fields set - only nil populated", func(t *testing.T) {
		spec := &v1alpha1.OrgQuotaParameters{
			Name: ptr.To("custom-name"),
			Orgs: []*string{ptr.To("existing-org")},
		}
		changed := LateInitialize(spec, fullResource)
		if !changed {
			t.Error("expected changed=true when some fields nil")
		}
		if ptr.Deref(spec.Name, "") != "custom-name" {
			t.Error("Name should not be overwritten")
		}
		if len(spec.Orgs) != 1 {
			t.Error("Orgs should not be overwritten when non-empty")
		}
		if ptr.Deref(spec.TotalServices, 0) != 5 {
			t.Error("TotalServices should be populated")
		}
	})

	t.Run("all fields set - no change", func(t *testing.T) {
		spec := &v1alpha1.OrgQuotaParameters{
			Name:                  ptr.To("custom-name"),
			AllowPaidServicePlans: ptr.To(false),
			InstanceMemory:        ptr.To(512.0),
			TotalAppInstances:     ptr.To(10.0),
			TotalAppLogRateLimit:  ptr.To(2048.0),
			TotalAppTasks:         ptr.To(4.0),
			TotalMemory:           ptr.To(1024.0),
			TotalPrivateDomains:   ptr.To(1.0),
			TotalRoutePorts:       ptr.To(2.0),
			TotalRoutes:           ptr.To(50.0),
			TotalServiceKeys:      ptr.To(5.0),
			TotalServices:         ptr.To(3.0),
			Orgs:                  []*string{ptr.To("existing-org")},
		}
		changed := LateInitialize(spec, fullResource)
		if changed {
			t.Error("expected changed=false when all fields set")
		}
	})
}
