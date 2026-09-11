package catalog

import "testing"

func TestAll_ReturnsACopy(t *testing.T) {
	got := All()
	if len(got) == 0 {
		t.Fatal("All() returned no rules; expected rules_src.go's init() to have registered some")
	}
	got[0].ID = "mutated"
	if All()[0].ID == "mutated" {
		t.Fatal("All() must return a copy; mutating the result corrupted the registry")
	}
}

func TestEvaluate_RunsEveryRegisteredRule(t *testing.T) {
	vm := VM{Name: "clean-vm-01"}
	results := Evaluate(vm)
	if len(results) != len(All()) {
		t.Fatalf("Evaluate() returned %d results, want %d (one per registered rule)", len(results), len(All()))
	}
}

func TestRegister_IDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range All() {
		if seen[r.ID] {
			t.Fatalf("duplicate rule ID %q registered", r.ID)
		}
		seen[r.ID] = true
	}
}
