// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/aggregates"
	"github.com/gophercloud/gophercloud/v2/openstack/compute/v2/flavors"
)

func TestFilterFlavorsByAggregateBinding(t *testing.T) {
	// Mirrors issue #2010: m1.xlarge is bound via aggregate_instance_extra_specs
	// to the "vjb-test" aggregate, which has no hosts. The migration target is
	// "vjb-simple", whose aggregate does have a host. Nova's
	// AggregateInstanceExtraSpecsFilter matches on aggregate METADATA only, so
	// vjbSimple and vjbTest are deliberately given different aggregate Names but
	// the SAME metadata key ("cluster") to prove the filter doesn't rely on name
	// or AZ string matching for the metadata comparison itself.
	vjbSimple := aggregates.Aggregate{
		Name:             "vjb-simple-aggregate",
		AvailabilityZone: "vjb-simple",
		Metadata:         map[string]string{"cluster": "vjb-simple"},
		Hosts:            []string{"host1"},
	}
	vjbTest := aggregates.Aggregate{
		Name:             "vjb-test-aggregate",
		AvailabilityZone: "vjb-test",
		Metadata:         map[string]string{"cluster": "vjb-test"},
		Hosts:            []string{}, // no hosts — the reproduction condition
	}
	allAggregates := []aggregates.Aggregate{vjbSimple, vjbTest}

	global := flavors.Flavor{ID: "global"}
	boundToSimple := flavors.Flavor{
		ID:         "bound-to-simple",
		ExtraSpecs: map[string]string{"aggregate_instance_extra_specs:cluster": "vjb-simple"},
	}
	boundToTest := flavors.Flavor{
		ID:         "bound-to-test",
		ExtraSpecs: map[string]string{"aggregate_instance_extra_specs:cluster": "vjb-test"},
	}
	allFlavors := []flavors.Flavor{global, boundToSimple, boundToTest}

	tests := []struct {
		name          string
		targetAZ      string
		allAggregates []aggregates.Aggregate
		want          []string
	}{
		{
			name:          "target vjb-simple excludes flavor bound to hostless vjb-test aggregate",
			targetAZ:      "vjb-simple",
			allAggregates: allAggregates,
			want:          []string{"global", "bound-to-simple"},
		},
		{
			name:          "empty targetAZ disables filtering",
			targetAZ:      "",
			allAggregates: allAggregates,
			want:          []string{"global", "bound-to-simple", "bound-to-test"},
		},
		{
			name:          "targetAZ matching no aggregate fails open",
			targetAZ:      "unknown-cluster",
			allAggregates: allAggregates,
			want:          []string{"global", "bound-to-simple", "bound-to-test"},
		},
		{
			name:          "targetAZ matching an aggregate with zero hosts fails open",
			targetAZ:      "vjb-test",
			allAggregates: allAggregates,
			want:          []string{"global", "bound-to-simple", "bound-to-test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterFlavorsByAggregateBinding(allFlavors, tt.targetAZ, tt.allAggregates)
			gotIDs := make([]string, len(got))
			for i, f := range got {
				gotIDs[i] = f.ID
			}
			if len(gotIDs) != len(tt.want) {
				t.Fatalf("got %v, want %v", gotIDs, tt.want)
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.want[i] {
					t.Errorf("got %v, want %v", gotIDs, tt.want)
					return
				}
			}
		})
	}
}

func TestFilterFlavorsByAggregateBinding_MultipleKeysOnDifferentAggregates(t *testing.T) {
	// Documents the accepted per-key-union simplification: a flavor requiring
	// two aggregate_instance_extra_specs keys passes if EACH key independently
	// matches some aggregate touching a target host, even if no single
	// aggregate satisfies both keys at once. True Nova per-host evaluation
	// would require one aggregate to satisfy all keys simultaneously; this is
	// a deliberate, simpler approximation.
	aggA := aggregates.Aggregate{
		Name:             "agg-a",
		AvailabilityZone: "target-az",
		Metadata:         map[string]string{"ssd": "true"},
		Hosts:            []string{"host1"},
	}
	aggB := aggregates.Aggregate{
		Name:             "agg-b",
		AvailabilityZone: "target-az",
		Metadata:         map[string]string{"rack": "a1"},
		Hosts:            []string{"host1"},
	}
	allAggregates := []aggregates.Aggregate{aggA, aggB}

	flavor := flavors.Flavor{
		ID: "needs-both",
		ExtraSpecs: map[string]string{
			"aggregate_instance_extra_specs:ssd":  "true",
			"aggregate_instance_extra_specs:rack": "a1",
		},
	}

	got := FilterFlavorsByAggregateBinding([]flavors.Flavor{flavor}, "target-az", allAggregates)
	if len(got) != 1 {
		t.Fatalf("expected flavor to pass under per-key union semantics, got %v", got)
	}
}

func TestFilterFlavorsByAggregateBinding_SharedHostAcrossAggregates(t *testing.T) {
	// A host can belong to multiple aggregates. The target AZ aggregate has
	// the host; a second, differently-named aggregate also contains that same
	// host and carries the metadata the flavor requires.
	azAggregate := aggregates.Aggregate{
		Name:             "az-aggregate",
		AvailabilityZone: "target-az",
		Metadata:         map[string]string{},
		Hosts:            []string{"host1"},
	}
	extraAggregate := aggregates.Aggregate{
		Name:             "extra-aggregate",
		AvailabilityZone: "",
		Metadata:         map[string]string{"gpu": "true"},
		Hosts:            []string{"host1"},
	}
	allAggregates := []aggregates.Aggregate{azAggregate, extraAggregate}

	flavor := flavors.Flavor{
		ID:         "needs-gpu-aggregate",
		ExtraSpecs: map[string]string{"aggregate_instance_extra_specs:gpu": "true"},
	}

	got := FilterFlavorsByAggregateBinding([]flavors.Flavor{flavor}, "target-az", allAggregates)
	if len(got) != 1 {
		t.Fatalf("expected flavor bound via a co-located aggregate to pass, got %v", got)
	}
}
