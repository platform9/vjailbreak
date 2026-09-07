package preflight

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestLoadRoleDefinition(t *testing.T) {
	rd, err := LoadRoleDefinition(filepath.Join("testdata", "role-granted.json"))
	if err != nil {
		t.Fatalf("LoadRoleDefinition() error = %v", err)
	}
	if rd.Name != "vassessment-readonly" {
		t.Errorf("Name = %q, want %q", rd.Name, "vassessment-readonly")
	}
	want := []string{"System.View", "System.Read", "Datastore.Browse"}
	if !reflect.DeepEqual(rd.Privileges, want) {
		t.Errorf("Privileges = %v, want %v", rd.Privileges, want)
	}
}

func TestLoadRoleDefinition_MissingFile(t *testing.T) {
	if _, err := LoadRoleDefinition(filepath.Join("testdata", "does-not-exist.json")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadRoleDefinition_MissingName(t *testing.T) {
	if _, err := LoadRoleDefinition(filepath.Join("testdata", "role-no-name.json")); err == nil {
		t.Fatal("expected error for role definition missing \"name\", got nil")
	}
}

func TestValidatePrivileges(t *testing.T) {
	cases := []struct {
		name        string
		required    []string
		granted     []string
		wantMissing []string
		wantExtra   []string
		wantOK      bool
	}{
		{
			name:     "exact match",
			required: []string{"System.View", "System.Read"},
			granted:  []string{"System.View", "System.Read"},
			wantOK:   true,
		},
		{
			name:        "missing one",
			required:    []string{"System.View", "System.Read", "Datastore.Browse"},
			granted:     []string{"System.View"},
			wantMissing: []string{"System.Read", "Datastore.Browse"},
			wantOK:      false,
		},
		{
			name:      "grants more than required",
			required:  []string{"System.View"},
			granted:   []string{"System.View", "VirtualMachine.Interact.PowerOn"},
			wantExtra: []string{"VirtualMachine.Interact.PowerOn"},
			wantOK:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			missing, extra, ok := ValidatePrivileges(c.required, c.granted)
			sort.Strings(missing)
			sort.Strings(c.wantMissing)
			if !reflect.DeepEqual(missing, c.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, c.wantMissing)
			}
			if !reflect.DeepEqual(extra, c.wantExtra) {
				t.Errorf("extra = %v, want %v", extra, c.wantExtra)
			}
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v", ok, c.wantOK)
			}
		})
	}
}
