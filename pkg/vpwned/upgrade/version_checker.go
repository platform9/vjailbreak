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

// Fetch all releases newer than the current version.
func CheckForUpdates(ctx context.Context, owner, repo, currentVersion string) ([]ReleaseInfo, error) {
	client := github.NewClient(nil)
	releases, _, err := client.Repositories.ListReleases(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	var availableUpgrades []ReleaseInfo
	for _, release := range releases {
		tagName := release.GetTagName()
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
	}

	sort.Slice(availableUpgrades, func(i, j int) bool {
		return semver.Compare(availableUpgrades[i].Version, availableUpgrades[j].Version) < 0
	})

	return availableUpgrades, nil
}
