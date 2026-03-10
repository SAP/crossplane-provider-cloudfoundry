/*
Copyright 2023 SAP SE
*/

package clients

import (
	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// NamespacedRefToRef converts a NamespacedReference to a Reference.
func NamespacedRefToRef(nr *xpv1.NamespacedReference) *xpv1.Reference {
	if nr == nil {
		return nil
	}
	return &xpv1.Reference{
		Name:   nr.Name,
		Policy: nr.Policy,
	}
}

// RefToNamespacedRef converts a Reference to a NamespacedReference.
func RefToNamespacedRef(r *xpv1.Reference) *xpv1.NamespacedReference {
	if r == nil {
		return nil
	}
	return &xpv1.NamespacedReference{
		Name:   r.Name,
		Policy: r.Policy,
	}
}

// NamespacedSelectorToSelector converts a NamespacedSelector to a Selector.
func NamespacedSelectorToSelector(ns *xpv1.NamespacedSelector) *xpv1.Selector {
	if ns == nil {
		return nil
	}
	return &xpv1.Selector{
		MatchLabels:        ns.MatchLabels,
		MatchControllerRef: ns.MatchControllerRef,
		Policy:             ns.Policy,
	}
}
