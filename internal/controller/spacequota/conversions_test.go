package spacequota

import (
	"testing"

	"k8s.io/utils/ptr"
)

const alpha = "alpha"
const beta = "beta"

func TestGetSpaceStatusHelper(t *testing.T) {
	t.Log("empty inputs")
	// We don't expect or observe any space assigned to the space quota.
	sStatus := getSpaceStatusHelper([]*string{}, []*string{})
	if l := len(sStatus); l != 0 {
		t.Errorf("unexpected sStatus len: %d", l)
	}
	// We shall not create any space binding.
	if l := len(sStatus.toCreate()); l != 0 {
		t.Errorf("unexpected sStatus.toCreate len: %d", l)
	}
	// We shall not delete any space binding.
	if l := len(sStatus.toDelete()); l != 0 {
		t.Errorf("unexpected sStatus.toDelete len: %d", l)
	}

	t.Log("single in spec")
	// We expect a single space but observe 0 spaces assigned
	// to the space quota.
	sStatus = getSpaceStatusHelper(
		[]*string{
			ptr.To(alpha),
		},
		[]*string{})
	if l := len(sStatus); l != 1 {
		t.Errorf("unexpected sStatus len: %d", l)
	}
	ss, found := sStatus[alpha]
	if !found {
		t.Errorf("alpha is not found")
	}
	if ss.inSpec != true {
		t.Errorf("alpha is not in spec")
	}
	if ss.inStatus != false {
		t.Errorf("alpha is in status")
	}
	// We shall create a single space binding
	if l := len(sStatus.toCreate()); l != 1 {
		t.Errorf("unexpected sStatus.toCreate len: %d", l)
	}
	// We shall not delete any space binding.
	if l := len(sStatus.toDelete()); l != 0 {
		t.Errorf("unexpected sStatus.toDelete len: %d", l)
	}

	t.Log("single in status")
	// We expect 0 spaces but observe a single space assigned
	// to the space quota.
	sStatus = getSpaceStatusHelper(
		[]*string{},
		[]*string{
			ptr.To(alpha),
		})
	if l := len(sStatus); l != 1 {
		t.Errorf("unexpected sStatus len: %d", l)
	}
	ss, found = sStatus[alpha]
	if !found {
		t.Errorf("alpha is not found")
	}
	if ss.inSpec != false {
		t.Errorf("alpha is in spec")
	}
	if ss.inStatus != true {
		t.Errorf("alpha is not in status")
	}
	// We shall not create any space binding.
	if l := len(sStatus.toCreate()); l != 0 {
		t.Errorf("unexpected sStatus.toCreate len: %d", l)
	}
	// We shall delete a space binding.
	if l := len(sStatus.toDelete()); l != 1 {
		t.Errorf("unexpected sStatus.toDelete len: %d", l)
	}

	t.Log("single in both")
	// We expect a single space observe the same single space
	// assigned to the space quota.
	sStatus = getSpaceStatusHelper(
		[]*string{
			ptr.To(alpha),
		},
		[]*string{
			ptr.To(alpha),
		})
	if l := len(sStatus); l != 1 {
		t.Errorf("unexpected sStatus len: %d", l)
	}
	ss, found = sStatus[alpha]
	if !found {
		t.Errorf("alpha is not found")
	}
	if ss.inSpec != true {
		t.Errorf("alpha is not in spec")
	}
	if ss.inStatus != true {
		t.Errorf("alpha is not in status")
	}
	// We shall not create any space binding.
	if l := len(sStatus.toCreate()); l != 0 {
		t.Errorf("unexpected sStatus.toCreate len: %d", l)
	}
	// We shall not delete any space binding.
	if l := len(sStatus.toDelete()); l != 0 {
		t.Errorf("unexpected sStatus.toDelete len: %d", l)
	}

	// We expect two spaces (alpha, beta) and observe two spaces
	// (alpha, gamma) assigned to the space quota.
	t.Log("multiple mixed")
	sStatus = getSpaceStatusHelper(
		[]*string{
			ptr.To(alpha),
			ptr.To(beta),
		},
		[]*string{
			ptr.To(alpha),
			ptr.To("gamma"),
		})
	if l := len(sStatus); l != 3 {
		t.Errorf("unexpected sStatus len: %d", l)
	}
	ss, found = sStatus[alpha]
	if !found {
		t.Errorf("alpha is not found")
	}
	if ss.inSpec != true {
		t.Errorf("alpha is not in spec")
	}
	if ss.inStatus != true {
		t.Errorf("alpha is not in status")
	}
	ss, found = sStatus[beta]
	if !found {
		t.Errorf("beta is not found")
	}
	if ss.inSpec != true {
		t.Errorf("beta is not in spec")
	}
	if ss.inStatus != false {
		t.Errorf("beta is in status")
	}
	ss, found = sStatus["gamma"]
	if !found {
		t.Errorf("gamma is not found")
	}
	if ss.inSpec != false {
		t.Errorf("gamma is in spec")
	}
	if ss.inStatus != true {
		t.Errorf("gamma is in not status")
	}
	// We shall create a single space binding.
	if l := len(sStatus.toCreate()); l != 1 {
		t.Errorf("unexpected sStatus.toCreate len: %d", l)
	}
	// We shall delete a single space binding.
	if l := len(sStatus.toDelete()); l != 1 {
		t.Errorf("unexpected sStatus.toDelete len: %d", l)
	}

}
