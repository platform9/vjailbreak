// Package utils provides utility functions for handling credentials and other operations
package utils

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// FetchAttachedTagsForVMs returns a map of VM MOID -> (tag category name -> comma-separated
// tag names) for all the given VM references using the vAPI tagging service. A nil map with
// an error means the tagging service could not be queried; callers should treat that as
// "unknown" and preserve any previously discovered tags.
func FetchAttachedTagsForVMs(ctx context.Context, c *vim25.Client, username, password string, refs []types.ManagedObjectReference) (map[string]map[string]string, error) {
	if len(refs) == 0 {
		return map[string]map[string]string{}, nil
	}

	restClient := rest.NewClient(c)
	if err := restClient.Login(ctx, url.UserPassword(username, password)); err != nil {
		return nil, fmt.Errorf("failed to login to vAPI tagging endpoint: %w", err)
	}
	defer func() {
		if err := restClient.Logout(ctx); err != nil {
			log.FromContext(ctx).Error(err, "failed to logout from vAPI tagging endpoint")
		}
	}()

	tagsManager := tags.NewManager(restClient)

	categories, err := tagsManager.GetCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list tag categories: %w", err)
	}
	categoryNames := make(map[string]string, len(categories))
	for _, category := range categories {
		categoryNames[category.ID] = category.Name
	}

	objRefs := make([]mo.Reference, 0, len(refs))
	for i := range refs {
		objRefs = append(objRefs, refs[i])
	}

	attached, err := tagsManager.GetAttachedTagsOnObjects(ctx, objRefs)
	if err != nil {
		return nil, fmt.Errorf("failed to get attached tags: %w", err)
	}

	result := make(map[string]map[string]string, len(refs))
	for _, entry := range attached {
		vmTags := BuildTagCategoryMap(entry.Tags, categoryNames)
		if len(vmTags) > 0 {
			result[entry.ObjectID.Reference().Value] = vmTags
		}
	}
	return result, nil
}

// BuildTagCategoryMap groups tag names by their category name, comma-joining multiple
// tags in the same category (e.g. "env" -> "staging,test"). Tags whose category is
// unknown are grouped under their raw category ID so no data is silently dropped.
func BuildTagCategoryMap(vmTags []tags.Tag, categoryNames map[string]string) map[string]string {
	if len(vmTags) == 0 {
		return nil
	}
	grouped := make(map[string]string, len(vmTags))
	for _, tag := range vmTags {
		categoryName, ok := categoryNames[tag.CategoryID]
		if !ok || categoryName == "" {
			categoryName = tag.CategoryID
		}
		if existing, ok := grouped[categoryName]; ok {
			grouped[categoryName] = existing + "," + tag.Name
		} else {
			grouped[categoryName] = tag.Name
		}
	}
	return grouped
}

// BuildSourceTagsMetadata flattens a VM's vSphere tags and custom attributes into
// OpenStack instance metadata keys. Tag categories are prefixed with "tag:" and
// custom attributes with "attr:" so source-derived keys stay distinguishable from
// user-entered custom metadata and cannot collide with each other.
func BuildSourceTagsMetadata(vmTags, customAttributes map[string]string) map[string]string {
	if len(vmTags) == 0 && len(customAttributes) == 0 {
		return nil
	}
	metadata := make(map[string]string, len(vmTags)+len(customAttributes))
	for category, tagNames := range vmTags {
		metadata["tag:"+category] = tagNames
	}
	for name, value := range customAttributes {
		metadata["attr:"+name] = value
	}
	return metadata
}

// ExtractCustomAttributes maps a VM's vSphere custom attribute values to their field
// names using the availableField definitions from the same VM properties. Attributes
// with empty values are skipped.
func ExtractCustomAttributes(vmProps *mo.VirtualMachine) map[string]string {
	if vmProps == nil || len(vmProps.CustomValue) == 0 {
		return nil
	}

	fieldNames := make(map[int32]string, len(vmProps.AvailableField))
	for _, field := range vmProps.AvailableField {
		fieldNames[field.Key] = field.Name
	}

	attributes := make(map[string]string, len(vmProps.CustomValue))
	for _, value := range vmProps.CustomValue {
		stringValue, ok := value.(*types.CustomFieldStringValue)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringValue.Value) == "" {
			continue
		}
		name, ok := fieldNames[stringValue.Key]
		if !ok || name == "" {
			continue
		}
		attributes[name] = stringValue.Value
	}
	if len(attributes) == 0 {
		return nil
	}
	return attributes
}
