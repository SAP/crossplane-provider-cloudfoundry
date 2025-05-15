package orgquota

import (
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// OrgQuota is the interface that defines the methods that a OrgQuota
// client should implement.
type OrgQuota interface {
	Get(ctx context.Context, guid string) (*resource.OrganizationQuota, error)
	Create(ctx context.Context, res *resource.OrganizationQuotaCreateOrUpdate) (*resource.OrganizationQuota, error)
	Update(ctx context.Context, guid string, r *resource.OrganizationQuotaCreateOrUpdate) (*resource.OrganizationQuota, error)
	Delete(ctx context.Context, guid string) (string, error)
}

// NewClient creates a new OrgQuota client
func NewClient(cf *client.Client) OrgQuota {
	return cf.OrganizationQuotas
}

// GenerateCreate generates the OrgazationQuotaCreateOrUpdate from
// OrgQuotaParameters. The float64 fields of spec with negative or nil
// values indicate unset values.
//
//nolint:gocyclo
func GenerateCreateOrUpdate(spec v1alpha1.OrgQuotaParameters) *resource.OrganizationQuotaCreateOrUpdate {
	name := ptr.Deref(spec.Name, "")
	createOrUpdate := resource.NewOrganizationQuotaCreate(name)
	if v := spec.AllowPaidServicePlans; v != nil {
		createOrUpdate = createOrUpdate.WithPaidServicesAllowed(*v)
	}
	if v := spec.InstanceMemory; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithPerProcessMemoryInMB(int(*v))
	}
	if v := spec.TotalAppInstances; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithTotalInstances(int(*v))
	}
	if v := spec.TotalAppLogRateLimit; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithLogRateLimitInBytesPerSecond(int(*v))
	}
	if v := spec.TotalAppTasks; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithPerAppTasks(int(*v))
	}
	if v := spec.TotalMemory; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithAppsTotalMemoryInMB(int(*v))
	}
	if v := spec.TotalPrivateDomains; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithDomains(int(*v))
	}
	if v := spec.TotalRoutePorts; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithTotalReservedPorts(int(*v))
	}
	if v := spec.TotalRoutes; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithTotalRoutes(int(*v))
	}
	if v := spec.TotalServiceKeys; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithTotalServiceKeys(int(*v))
	}
	if v := spec.TotalServices; v != nil && *v >= 0.0 {
		createOrUpdate = createOrUpdate.WithTotalServiceInstances(int(*v))
	}
	orgs := make([]string, 0, len(spec.Orgs))
	for _, org := range spec.Orgs {
		if org != nil {
			orgs = append(orgs, *org)
		}
	}
	createOrUpdate.WithOrganizations(orgs...)
	return createOrUpdate
}

// intpToFloatp function takes an *int value and turns it into a
// *float64 value. If in is nil, the function returns nil.
func intpToFloatp(in *int) *float64 {
	if in == nil {
		return nil
	}
	return ptr.To(float64(*in))
}

// GenerateObservation function takes an OrganizationQuota resource
// and returns an OrgQuotaObservation.
func GenerateObservation(o *resource.OrganizationQuota) v1alpha1.OrgQuotaObservation {
	obs := v1alpha1.OrgQuotaObservation{
		AllowPaidServicePlans: ptr.To(o.Services.PaidServicesAllowed),
		CreatedAt:             ptr.To(o.CreatedAt.Format(time.RFC3339)),
		ID:                    ptr.To(o.GUID),
		InstanceMemory:        intpToFloatp(o.Apps.PerProcessMemoryInMB),
		Name:                  ptr.To(o.Name),
		Orgs:                  make([]*string, len(o.Relationships.Organizations.Data)),
		TotalAppInstances:     intpToFloatp(o.Apps.TotalInstances),
		TotalAppLogRateLimit:  intpToFloatp(o.Apps.LogRateLimitInBytesPerSecond),
		TotalAppTasks:         intpToFloatp(o.Apps.PerAppTasks),
		TotalMemory:           intpToFloatp(o.Apps.TotalMemoryInMB),
		TotalPrivateDomains:   intpToFloatp(o.Domains.TotalDomains),
		TotalRoutePorts:       intpToFloatp(o.Routes.TotalReservedPorts),
		TotalRoutes:           intpToFloatp(o.Routes.TotalRoutes),
		TotalServiceKeys:      intpToFloatp(o.Services.TotalServiceKeys),
		TotalServices:         intpToFloatp(o.Services.TotalServiceInstances),
		UpdatedAt:             ptr.To(o.UpdatedAt.Format(time.RFC3339)),
	}

	for i, orgData := range o.Relationships.Organizations.Data {
		obs.Orgs[i] = ptr.To(orgData.GUID)
	}
	return obs
}

// orgsEqual compares two *string slices. The nil values of
// the slices are ignored. The order of the values in the two slices
// are indifferent.
//
// The two slices are equal if they contain the same values.
func orgsEqual(orgs1, orgs2 []*string) bool {
	// orgSet1 is a set that contains non-nil strings of org1
	orgSet1 := map[string]interface{}{}
	for _, org := range orgs1 {
		if org != nil {
			if _, ok := orgSet1[*org]; ok {
				// org name is listed twice
				continue
			}
			orgSet1[*org] = struct{}{}
		}
	}
	// orgSet2 is a set that contains non-nil strings of org1
	orgSet2 := map[string]interface{}{}
	for _, org := range orgs2 {
		if org != nil {
			if _, ok := orgSet2[*org]; ok {
				// org name is listed twice
				continue
			}
			orgSet2[*org] = struct{}{}
		}
	}
	return maps.Equal(orgSet1, orgSet2)
}

// NeedsReconciliation function investigates a managed OrgQuota
// resource. It compares the Spec.ForProvider object with the
// Status.AtProvider.
//
//nolint:gocyclo
func NeedsReconciliation(orgQuota *v1alpha1.OrgQuota) bool {
	if ptr.Deref(orgQuota.Spec.ForProvider.Name, "") != ptr.Deref(orgQuota.Status.AtProvider.Name, "") ||
		!ptr.Equal(orgQuota.Spec.ForProvider.AllowPaidServicePlans, orgQuota.Status.AtProvider.AllowPaidServicePlans) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.InstanceMemory, orgQuota.Status.AtProvider.InstanceMemory) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalAppInstances, orgQuota.Status.AtProvider.TotalAppInstances) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalAppLogRateLimit, orgQuota.Status.AtProvider.TotalAppLogRateLimit) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalAppTasks, orgQuota.Status.AtProvider.TotalAppTasks) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalMemory, orgQuota.Status.AtProvider.TotalMemory) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalPrivateDomains, orgQuota.Status.AtProvider.TotalPrivateDomains) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalRoutePorts, orgQuota.Status.AtProvider.TotalRoutePorts) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalRoutes, orgQuota.Status.AtProvider.TotalRoutes) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalServiceKeys, orgQuota.Status.AtProvider.TotalServiceKeys) ||
		!ptr.Equal(orgQuota.Spec.ForProvider.TotalServices, orgQuota.Status.AtProvider.TotalServices) ||
		orgsEqual(orgQuota.Spec.ForProvider.Orgs, orgQuota.Status.AtProvider.Orgs) {
		return true
	}
	return false
}

// ptrCast generic function takes an in *ptr value and a default
// value. It dereferences first in. If in is nil, it takes the default
// value. Then it casts the value to another type and returns with a
// pointer to that type.
//
// For example ptrCast[int, float64] will accept an *int and returns
// *float64.
func ptrCast[I, O interface{ ~int | ~float64 | ~float32 }](in *I, defValue I) *O {
	return ptr.To(O(ptr.Deref(in, defValue)))
}

// ptrDef function accepts a pointer to a value. If in is not nil,
// then it returns with in. Otherwise, it returns with a pointer to
// default value.
func ptrDef[T any](in *T, defValue T) *T {
	return ptr.To(ptr.Deref(in, defValue))
}

// LateInitialize fills the unassigned fields with values from a
// OrganizationQuota resource.
//
//nolint:gocyclo
func LateInitialize(spec *v1alpha1.OrgQuotaParameters, from *resource.OrganizationQuota) bool {
	slog.Info("LateInitialize invoked")
	changed := false
	if spec.Name == nil {
		spec.Name = &from.Name
		changed = true
	}
	if len(spec.Orgs) == 0 {
		spec.Orgs = make([]*string, len(from.Relationships.Organizations.Data))
		for i := range from.Relationships.Organizations.Data {
			spec.Orgs[i] = &from.Relationships.Organizations.Data[i].GUID
		}
		changed = true
	}
	if spec.AllowPaidServicePlans == nil {
		spec.AllowPaidServicePlans = ptr.To(from.Services.PaidServicesAllowed)
		changed = true
	}
	if spec.InstanceMemory == nil {
		spec.InstanceMemory = ptrCast[int, float64](from.Apps.PerProcessMemoryInMB, -1)
		changed = true
	}
	if spec.TotalAppInstances == nil {
		spec.TotalAppInstances = ptrCast[int, float64](from.Apps.TotalInstances, -1)
		changed = true
	}
	if spec.TotalAppLogRateLimit == nil {
		spec.TotalAppInstances = ptrCast[int, float64](from.Apps.LogRateLimitInBytesPerSecond, -1)
		changed = true
	}
	if spec.TotalAppTasks == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Apps.PerAppTasks, -1)
		changed = true
	}
	if spec.TotalMemory == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Apps.TotalMemoryInMB, -1)
		changed = true
	}
	if spec.TotalPrivateDomains == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Domains.TotalDomains, -1)
		changed = true
	}
	if spec.TotalRoutePorts == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Routes.TotalReservedPorts, -1)
		changed = true
	}
	if spec.TotalRoutes == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Routes.TotalRoutes, -1)
		changed = true
	}
	if spec.TotalServiceKeys == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Services.TotalServiceKeys, -1)
		changed = true
	}
	if spec.TotalServices == nil {
		spec.TotalAppTasks = ptrCast[int, float64](from.Services.TotalServiceInstances, -1)
		changed = true
	}
	slog.Info("LateInitialize done", "changed", changed)
	return changed
}
