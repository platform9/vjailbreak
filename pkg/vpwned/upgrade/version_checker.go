package upgrade

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/go-github/v63/github"
	"golang.org/x/mod/semver"
)

type ReleaseInfo struct {
	Version      string
	ReleaseNotes string
	DownloadURL  string
}

// Returns all tags for dropdown
func GetAllTags(ctx context.Context, owner, repo string) ([]string, error) {
	client := github.NewClient(nil)
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tagNames []string
	for _, tag := range tags {
		tagNames = append(tagNames, tag.GetName())
	}
	sort.Slice(tagNames, func(i, j int) bool {
		return semver.Compare(tagNames[i], tagNames[j]) < 0
	})
	return tagNames, nil
}

// Fetch all releases newer than the current version if semver, else all tags.
func CheckForUpdates(ctx context.Context, owner, repo, currentVersion string) ([]ReleaseInfo, error) {
	client := github.NewClient(nil)
	releases, _, err := client.Repositories.ListReleases(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	var availableUpgrades []ReleaseInfo
	isSemver := semver.IsValid(currentVersion)
	for _, release := range releases {
		tagName := release.GetTagName()
		if isSemver {
			if semver.Compare(tagName, currentVersion) > 0 {
				info := ReleaseInfo{
					Version:      tagName,
					ReleaseNotes: release.GetBody(),
				}
				if len(release.Assets) > 0 {
					info.DownloadURL = release.Assets[0].GetBrowserDownloadURL()
				}
				availableUpgrades = append(availableUpgrades, info)
			}
		} else {
			info := ReleaseInfo{
				Version:      tagName,
				ReleaseNotes: release.GetBody(),
			}
			if len(release.Assets) > 0 {
				info.DownloadURL = release.Assets[0].GetBrowserDownloadURL()
			}
			availableUpgrades = append(availableUpgrades, info)
		}
	}

	sort.Slice(availableUpgrades, func(i, j int) bool {
		return semver.Compare(availableUpgrades[i].Version, availableUpgrades[j].Version) < 0
	})

	return availableUpgrades, nil
}
