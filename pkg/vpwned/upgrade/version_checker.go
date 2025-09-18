package upgrade

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sort"

	"github.com/google/go-github/v63/github"
	"golang.org/x/mod/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ReleaseInfo struct {
	Version      string
	ReleaseNotes string
	DownloadURL  string
}

func getCurrentVersionFromConfigMap() (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	configMap, err := clientset.CoreV1().ConfigMaps("migration-system").Get(context.Background(), "version-config", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get version-config ConfigMap: %w", err)
	}

	version, exists := configMap.Data["version"]
	if !exists {
		return "", fmt.Errorf("version field not found in configmap")
	}

	return version, nil
}

func GetAllTags(ctx context.Context) ([]string, error) {
	owner, repo := loadGitHubConfig(ctx)
	currentVersion, err := getCurrentVersionFromConfigMap()
	if err != nil {
		fmt.Printf("Warning: Could not get current version from configmap: %v. Showing all tags.\n", err)
		return getAllTagsFromGitHub(ctx, owner, repo)
	}

	isSemver := semver.IsValid(currentVersion)

	if isSemver {
		fmt.Printf("Current version %s is semver format. Showing only newer versions.\n", currentVersion)
		return getTagsGreaterThanVersion(ctx, owner, repo, currentVersion)
	} else {
		fmt.Printf("Current version %s is not semver format. Showing all tags.\n", currentVersion)
		return getAllTagsFromGitHub(ctx, owner, repo)
	}
}

func getAllTagsFromGitHub(ctx context.Context, owner, repo string) ([]string, error) {
	client := github.NewClient(nil)
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags for repo %s/%s: %w", owner, repo, err)
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

func getTagsGreaterThanVersion(ctx context.Context, owner, repo, currentVersion string) ([]string, error) {
	client := github.NewClient(nil)
	tags, _, err := client.Repositories.ListTags(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags for repo %s/%s: %w", owner, repo, err)
	}

	var newerTagNames []string
	for _, tag := range tags {
		tagName := tag.GetName()
		if semver.Compare(tagName, currentVersion) > 0 {
			newerTagNames = append(newerTagNames, tagName)
		}
	}

	sort.Slice(newerTagNames, func(i, j int) bool {
		return semver.Compare(newerTagNames[i], newerTagNames[j]) < 0
	})

	return newerTagNames, nil
}

func loadGitHubConfig(ctx context.Context) (string, string) {
	owner := "platform9"
	repo := "vjailbreak"

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Warning: Could not get in-cluster config. Using default GitHub repo. Error: %v", err)
		return owner, repo
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Warning: Could not create kubernetes client. Using default GitHub repo. Error: %v", err)
		return owner, repo
	}

	configMap, err := clientset.CoreV1().ConfigMaps("migration-system").Get(ctx, "version-config", metav1.GetOptions{})
	if err != nil {
		log.Printf("Warning: Could not get version-config ConfigMap. Using default GitHub repo. Error: %v", err)
		return owner, repo
	}

	if val, ok := configMap.Data["githubOwner"]; ok && val != "" {
		owner = val
	}
	if val, ok := configMap.Data["githubRepo"]; ok && val != "" {
		repo = val
	}

	log.Printf("Using GitHub repository: %s/%s", owner, repo)
	return owner, repo
}

func CheckImagesExist(ctx context.Context, tag string) (bool, error) {
	log.Printf("Verifying images exist for tag: %s", tag)

	images := []string{
		"quay.io/platform9/vjailbreak-ui:" + tag,
		"quay.io/platform9/vjailbreak-controller:" + tag,
		"quay.io/platform9/vjailbreak-vpwned:" + tag,
	}

	for _, imageName := range images {
		cmd := exec.CommandContext(ctx, "skopeo", "inspect", "docker://"+imageName)
		if err := cmd.Run(); err != nil {
			log.Printf("Image check failed for %s: %v", imageName, err)
			return false, fmt.Errorf("required image not found: %s", imageName)
		}
		log.Printf("Image verified: %s", imageName)
	}

	return true, nil
}
