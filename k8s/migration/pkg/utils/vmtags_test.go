package utils

import (
	"reflect"
	"testing"

	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestBuildTagCategoryMap(t *testing.T) {
	categoryNames := map[string]string{
		"urn:cat:env":    "env",
		"urn:cat:tier":   "tier",
		"urn:cat:backup": "backup",
	}

	tests := []struct {
		name     string
		vmTags   []tags.Tag
		expected map[string]string
	}{
		{
			name:     "no tags returns nil",
			vmTags:   nil,
			expected: nil,
		},
		{
			name: "single tag",
			vmTags: []tags.Tag{
				{Name: "production", CategoryID: "urn:cat:env"},
			},
			expected: map[string]string{"env": "production"},
		},
		{
			name: "multiple categories",
			vmTags: []tags.Tag{
				{Name: "production", CategoryID: "urn:cat:env"},
				{Name: "web", CategoryID: "urn:cat:tier"},
			},
			expected: map[string]string{"env": "production", "tier": "web"},
		},
		{
			name: "multiple tags in same category are comma-joined",
			vmTags: []tags.Tag{
				{Name: "staging", CategoryID: "urn:cat:env"},
				{Name: "test", CategoryID: "urn:cat:env"},
			},
			expected: map[string]string{"env": "staging,test"},
		},
		{
			name: "unknown category falls back to category ID",
			vmTags: []tags.Tag{
				{Name: "orphan", CategoryID: "urn:cat:deleted"},
			},
			expected: map[string]string{"urn:cat:deleted": "orphan"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTagCategoryMap(tt.vmTags, categoryNames)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("BuildTagCategoryMap() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestExtractCustomAttributes(t *testing.T) {
	tests := []struct {
		name     string
		vmProps  *mo.VirtualMachine
		expected map[string]string
	}{
		{
			name:     "nil props returns nil",
			vmProps:  nil,
			expected: nil,
		},
		{
			name:     "no custom values returns nil",
			vmProps:  &mo.VirtualMachine{},
			expected: nil,
		},
		{
			name: "values mapped to field names",
			vmProps: &mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{
					ExtensibleManagedObject: mo.ExtensibleManagedObject{
						AvailableField: []types.CustomFieldDef{
							{Key: 101, Name: "Owner"},
							{Key: 102, Name: "CostCenter"},
						},
					},
					CustomValue: []types.BaseCustomFieldValue{
						&types.CustomFieldStringValue{
							CustomFieldValue: types.CustomFieldValue{Key: 101},
							Value:            "alice@corp.com",
						},
						&types.CustomFieldStringValue{
							CustomFieldValue: types.CustomFieldValue{Key: 102},
							Value:            "CC-1042",
						},
					},
				},
			},
			expected: map[string]string{"Owner": "alice@corp.com", "CostCenter": "CC-1042"},
		},
		{
			name: "empty values and unknown keys are skipped",
			vmProps: &mo.VirtualMachine{
				ManagedEntity: mo.ManagedEntity{
					ExtensibleManagedObject: mo.ExtensibleManagedObject{
						AvailableField: []types.CustomFieldDef{
							{Key: 101, Name: "Owner"},
						},
					},
					CustomValue: []types.BaseCustomFieldValue{
						&types.CustomFieldStringValue{
							CustomFieldValue: types.CustomFieldValue{Key: 101},
							Value:            "   ",
						},
						&types.CustomFieldStringValue{
							CustomFieldValue: types.CustomFieldValue{Key: 999},
							Value:            "no-field-def",
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCustomAttributes(tt.vmProps)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ExtractCustomAttributes() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildSourceTagsMetadata(t *testing.T) {
	tests := []struct {
		name             string
		vmTags           map[string]string
		customAttributes map[string]string
		expected         map[string]string
	}{
		{
			name:     "both empty returns nil",
			expected: nil,
		},
		{
			name:   "tags get tag prefix",
			vmTags: map[string]string{"env": "production", "tier": "web"},
			expected: map[string]string{
				"tag:env":  "production",
				"tag:tier": "web",
			},
		},
		{
			name:             "attributes get attr prefix",
			customAttributes: map[string]string{"Owner": "alice@corp.com"},
			expected:         map[string]string{"attr:Owner": "alice@corp.com"},
		},
		{
			name:             "same name in tags and attributes cannot collide",
			vmTags:           map[string]string{"Owner": "team-a"},
			customAttributes: map[string]string{"Owner": "alice@corp.com"},
			expected: map[string]string{
				"tag:Owner":  "team-a",
				"attr:Owner": "alice@corp.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildSourceTagsMetadata(tt.vmTags, tt.customAttributes)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("BuildSourceTagsMetadata() = %v, want %v", got, tt.expected)
			}
		})
	}
}
