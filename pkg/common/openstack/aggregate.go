// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"strings"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/aggregates"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
)

// AggregateInstanceExtraSpecsPrefix is the flavor extra_spec prefix Nova's
// AggregateInstanceExtraSpecsFilter matches against host-aggregate metadata.
// The suffix after the colon is an arbitrary, admin-defined key (not fixed,
// unlike AvailabilityZoneExtraSpecKey) — e.g. "aggregate_instance_extra_specs:ssd".
const AggregateInstanceExtraSpecsPrefix = "aggregate_instance_extra_specs:"

// FilterFlavorsByAggregateBinding returns the subset of flavors that can
// schedule onto targetAZ under Nova's AggregateInstanceExtraSpecsFilter.
//
// Nova's filter matches purely on aggregate METADATA, independent of an
// aggregate's Name or AvailabilityZone field. So resolving "which hosts back
// the target cluster" goes through host membership: the target hosts are the
// union of Hosts from every aggregate whose AvailabilityZone equals targetAZ
// (the same AZ value vjailbreak already passes to Nova's create-server call).
// A flavor's "aggregate_instance_extra_specs:<key>=<value>" extra_spec is
// satisfied if some aggregate sharing a target host has Metadata[<key>] ==
// <value>; each required key is checked independently (per-key union), not
// requiring one single aggregate to satisfy every key at once — a flavor
// requiring two keys held by two disjoint aggregates over the target hosts
// can pass here even though no single host in the target cluster actually
// satisfies both simultaneously, unlike Nova's real per-host evaluation.
//
// Only plain equality is evaluated. Nova's AggregateInstanceExtraSpecsFilter
// also accepts the ComputeCapabilitiesFilter operator grammar on the flavor's
// extra_spec value (e.g. "s==", "s!=", "<in>", ">="): an operator-valued
// binding is compared here as a literal string against the aggregate's plain
// metadata value, so it will not match and the flavor will be excluded even
// where Nova's real filter would have accepted it. Plain key=value bindings
// (the form in issue #2010) are unaffected.
//
// If targetAZ is empty, or matches no aggregate (or that aggregate has no
// hosts), the target host set cannot be determined and filtering is disabled
// (fails open) rather than risk excluding valid flavors on non-PCD or
// non-aggregate-based deployments.
func FilterFlavorsByAggregateBinding(allFlavors []flavors.Flavor, targetAZ string, allAggregates []aggregates.Aggregate) []flavors.Flavor {
	if targetAZ == "" {
		return allFlavors
	}

	targetHosts := make(map[string]struct{})
	for _, agg := range allAggregates {
		if agg.AvailabilityZone != targetAZ {
			continue
		}
		for _, host := range agg.Hosts {
			targetHosts[host] = struct{}{}
		}
	}
	if len(targetHosts) == 0 {
		return allFlavors
	}

	// candidateMetadata[key] is the set of values seen, among aggregates that
	// share at least one host with the target cluster, for that metadata key.
	candidateMetadata := make(map[string]map[string]struct{})
	for _, agg := range allAggregates {
		if !sharesHost(agg.Hosts, targetHosts) {
			continue
		}
		for key, value := range agg.Metadata {
			if candidateMetadata[key] == nil {
				candidateMetadata[key] = make(map[string]struct{})
			}
			candidateMetadata[key][value] = struct{}{}
		}
	}

	filtered := make([]flavors.Flavor, 0, len(allFlavors))
	for _, flavor := range allFlavors {
		if flavorMatchesAggregateBinding(flavor, candidateMetadata) {
			filtered = append(filtered, flavor)
		}
	}
	return filtered
}

func sharesHost(hosts []string, targetHosts map[string]struct{}) bool {
	for _, host := range hosts {
		if _, ok := targetHosts[host]; ok {
			return true
		}
	}
	return false
}

func flavorMatchesAggregateBinding(flavor flavors.Flavor, candidateMetadata map[string]map[string]struct{}) bool {
	for specKey, requiredValue := range flavor.ExtraSpecs {
		key, ok := strings.CutPrefix(specKey, AggregateInstanceExtraSpecsPrefix)
		if !ok {
			continue
		}
		values, hasKey := candidateMetadata[key]
		if !hasKey {
			return false
		}
		if _, matches := values[requiredValue]; !matches {
			return false
		}
	}
	return true
}
