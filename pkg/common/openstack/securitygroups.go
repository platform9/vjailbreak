// Copyright © 2024 The vjailbreak authors

package openstack

import (
	"context"
	"fmt"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/projects"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/rbacpolicies"
	"github.com/gophercloud/gophercloud/v2/openstack/networking/v2/extensions/security/groups"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

// GetAccessibleSecurityGroups returns the security groups accessible to the project with the given name
func GetAccessibleSecurityGroups(ctx context.Context, networkingClient *gophercloud.ServiceClient, projectName string) ([]groups.SecGroup, error) {
	if projectName == "" {
		return nil, fmt.Errorf("projectName is required for security group lookup")
	}

	projectID, err := resolveProjectID(ctx, networkingClient.ProviderClient, projectName)
	if err != nil {
		return nil, err
	}

	ownedPages, err := groups.List(networkingClient, groups.ListOpts{
		TenantID: projectID,
	}).AllPages(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list owned security groups for project %s: %w", projectName, err)
	}
	accessibleGroups, err := groups.ExtractGroups(ownedPages)
	if err != nil {
		return nil, fmt.Errorf("failed to extract owned security groups: %w", err)
	}

	seenIDs := make(map[string]struct{}, len(accessibleGroups))
	for _, sg := range accessibleGroups {
		seenIDs[sg.ID] = struct{}{}
	}

	for _, target := range []string{projectID, "*"} {
		rbacPages, rbacErr := rbacpolicies.List(networkingClient, rbacpolicies.ListOpts{
			ObjectType:   "security_group",
			Action:       rbacpolicies.ActionAccessShared,
			TargetTenant: target,
		}).AllPages(ctx)
		if rbacErr != nil {
			continue
		}
		policies, rbacErr := rbacpolicies.ExtractRBACPolicies(rbacPages)
		if rbacErr != nil {
			continue
		}
		for _, policy := range policies {
			if _, seen := seenIDs[policy.ObjectID]; seen {
				continue
			}
			sg, fetchErr := groups.Get(ctx, networkingClient, policy.ObjectID).Extract()
			if fetchErr != nil {
				continue
			}
			accessibleGroups = append(accessibleGroups, *sg)
			seenIDs[policy.ObjectID] = struct{}{}
		}
	}

	return accessibleGroups, nil
}

// ListSecurityGroupInfos returns SecurityGroupInfo entries for all security groups
func ListSecurityGroupInfos(ctx context.Context, networkingClient *gophercloud.ServiceClient, projectName string) ([]vjailbreakv1alpha1.SecurityGroupInfo, error) {
	accessibleGroups, err := GetAccessibleSecurityGroups(ctx, networkingClient, projectName)
	if err != nil {
		return nil, err
	}

	nameCounts := make(map[string]int, len(accessibleGroups))
	for _, sg := range accessibleGroups {
		nameCounts[sg.Name]++
	}

	infos := make([]vjailbreakv1alpha1.SecurityGroupInfo, 0, len(accessibleGroups))
	for _, sg := range accessibleGroups {
		infos = append(infos, vjailbreakv1alpha1.SecurityGroupInfo{
			Name:              sg.Name,
			ID:                sg.ID,
			RequiresIDDisplay: nameCounts[sg.Name] > 1,
		})
	}
	return infos, nil
}

func resolveProjectID(ctx context.Context, providerClient *gophercloud.ProviderClient, projectName string) (string, error) {
	identityClient, err := openstack.NewIdentityV3(providerClient, gophercloud.EndpointOpts{})
	if err != nil {
		return "", fmt.Errorf("failed to create identity client: %w", err)
	}
	pages, err := projects.List(identityClient, projects.ListOpts{Name: projectName}).AllPages(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list projects with name %s: %w", projectName, err)
	}
	allProjects, err := projects.ExtractProjects(pages)
	if err != nil {
		return "", fmt.Errorf("failed to extract projects: %w", err)
	}
	if len(allProjects) == 0 {
		return "", fmt.Errorf("no project found with name %s", projectName)
	}
	return allProjects[0].ID, nil
}
