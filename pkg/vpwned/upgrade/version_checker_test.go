package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-github/v63/github"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

// serveGitHubTags points githubClientFactory at a stub API returning the given tag
// names, so tag listing is exercised without touching api.github.com.
func serveGitHubTags(t *testing.T, tagNames []string) {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/tags") {
			http.NotFound(w, r)
			return
		}
		var items []string
		for _, name := range tagNames {
			items = append(items, fmt.Sprintf(`{"name":%q}`, name))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "[%s]", strings.Join(items, ","))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	stubGitHubClient(t, server.URL)
}

// stubGitHubClient repoints githubClientFactory at baseURL. go-github requires the base
// URL to end in a slash.
func stubGitHubClient(t *testing.T, baseURL string) {
	t.Helper()

	parsed, err := url.Parse(baseURL + "/")
	if err != nil {
		t.Fatalf("failed to parse stub server URL: %v", err)
	}

	original := githubClientFactory
	githubClientFactory = func(context.Context) *github.Client {
		c := github.NewClient(nil)
		c.BaseURL = parsed
		return c
	}
	t.Cleanup(func() { githubClientFactory = original })
}

// serveGitHubError makes every tag request fail, covering the error branches.
func serveGitHubError(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	stubGitHubClient(t, server.URL)
}

func TestNormalizeSemver(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want string
	}{
		{name: "already prefixed", tag: "v0.4.8", want: "v0.4.8"},
		{name: "bare version gets prefix", tag: "0.4.8", want: "v0.4.8"},
		{name: "empty string gets prefix", tag: "", want: "v"},
		{name: "non-semver tag is left alone apart from prefix", tag: "main-abc1234", want: "vmain-abc1234"},
		{name: "prerelease is preserved", tag: "v0.4.9-rc1", want: "v0.4.9-rc1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSemver(tt.tag); got != tt.want {
				t.Errorf("normalizeSemver(%q) = %q, want %q", tt.tag, got, tt.want)
			}
		})
	}
}

func TestNewGitHubClientUsesTokenWhenPresent(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	if c := newGitHubClient(context.Background()); c == nil {
		t.Fatal("newGitHubClient() = nil without a token, want a client")
	}

	t.Setenv("GITHUB_TOKEN", "secret-token")
	if c := newGitHubClient(context.Background()); c == nil {
		t.Fatal("newGitHubClient() = nil with a token, want a client")
	}
}

func TestGetCurrentVersion(t *testing.T) {
	tests := []struct {
		name      string
		configMap *corev1.ConfigMap
		want      string
		wantErr   string
	}{
		{
			name: "reads the version key",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: "migration-system"},
				Data:       map[string]string{"version": "v0.4.8"},
			},
			want: "v0.4.8",
		},
		{
			name: "missing version key is an error",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: "migration-system"},
				Data:       map[string]string{"upgradeAvailable": "true"},
			},
			wantErr: "version field not found",
		},
		{
			name:    "missing ConfigMap is an error",
			wantErr: "failed to get version-config ConfigMap",
		},
		{
			name: "ConfigMap in another namespace is not used",
			configMap: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "version-config", Namespace: "default"},
				Data:       map[string]string{"version": "v9.9.9"},
			},
			wantErr: "failed to get version-config ConfigMap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var clientset *k8sfake.Clientset
			if tt.configMap != nil {
				clientset = k8sfake.NewSimpleClientset(tt.configMap)
			} else {
				clientset = k8sfake.NewSimpleClientset()
			}

			got, err := GetCurrentVersion(context.Background(), clientset)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("GetCurrentVersion() error = nil, want error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("GetCurrentVersion() error = %q, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetCurrentVersion() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("GetCurrentVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetAllTagsFromGitHubSorting(t *testing.T) {
	tests := []struct {
		name  string
		tags  []string
		want  []string
		about string
	}{
		{
			name: "semver tags sort by version, not lexically",
			tags: []string{"v0.4.10", "v0.4.9", "v0.4.2"},
			want: []string{"v0.4.2", "v0.4.9", "v0.4.10"},
		},
		{
			name:  "semver tags come before non-semver, which sort alphabetically",
			tags:  []string{"zeta-branch", "v0.4.8", "alpha-branch", "v0.3.8"},
			want:  []string{"v0.3.8", "v0.4.8", "alpha-branch", "zeta-branch"},
			about: "release dropdowns list real releases first",
		},
		{
			name: "unprefixed versions are treated as semver",
			tags: []string{"0.4.9", "v0.4.8"},
			want: []string{"v0.4.8", "0.4.9"},
		},
		{
			name: "no tags yields no error",
			tags: nil,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serveGitHubTags(t, tt.tags)

			got, err := getAllTagsFromGitHub(context.Background(), "platform9", "vjailbreak")
			if err != nil {
				t.Fatalf("getAllTagsFromGitHub() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getAllTagsFromGitHub() = %v, want %v (%s)", got, tt.want, tt.about)
			}
		})
	}
}

func TestGetAllTagsFromGitHubError(t *testing.T) {
	serveGitHubError(t)

	if _, err := getAllTagsFromGitHub(context.Background(), "platform9", "vjailbreak"); err == nil {
		t.Fatal("getAllTagsFromGitHub() error = nil, want an error when the API fails")
	}
}

func TestGetTagsGreaterThanVersion(t *testing.T) {
	tests := []struct {
		name           string
		tags           []string
		currentVersion string
		want           []string
	}{
		{
			name:           "only newer versions, ascending",
			tags:           []string{"v0.4.7", "v0.4.8", "v0.4.9", "v0.5.0"},
			currentVersion: "v0.4.8",
			want:           []string{"v0.4.9", "v0.5.0"},
		},
		{
			name:           "the current version itself is excluded",
			tags:           []string{"v0.4.8"},
			currentVersion: "v0.4.8",
			want:           nil,
		},
		{
			name:           "non-semver tags are skipped",
			tags:           []string{"main-abc1234", "v0.4.9", "some-branch"},
			currentVersion: "v0.4.8",
			want:           []string{"v0.4.9"},
		},
		{
			name:           "unprefixed tags are normalized before comparing",
			tags:           []string{"0.4.9", "0.4.7"},
			currentVersion: "0.4.8",
			want:           []string{"v0.4.9"},
		},
		{
			name:           "patch ordering is numeric",
			tags:           []string{"v0.4.10", "v0.4.9"},
			currentVersion: "v0.4.8",
			want:           []string{"v0.4.9", "v0.4.10"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serveGitHubTags(t, tt.tags)

			got, err := getTagsGreaterThanVersion(context.Background(), "platform9", "vjailbreak", tt.currentVersion)
			if err != nil {
				t.Fatalf("getTagsGreaterThanVersion() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTagsGreaterThanVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTagsGreaterThanVersionError(t *testing.T) {
	serveGitHubError(t)

	if _, err := getTagsGreaterThanVersion(context.Background(), "platform9", "vjailbreak", "v0.4.8"); err == nil {
		t.Fatal("getTagsGreaterThanVersion() error = nil, want an error when the API fails")
	}
}

// Outside a cluster there is no in-cluster config, so the defaults must be returned
// rather than an empty owner/repo that would build a broken GitHub URL.
func TestLoadGitHubConfigFallsBackToDefaults(t *testing.T) {
	owner, repo := loadGitHubConfig(context.Background())

	if owner != "platform9" || repo != "vjailbreak" {
		t.Errorf("loadGitHubConfig() = %q/%q, want platform9/vjailbreak", owner, repo)
	}
}

// GetAllTags cannot reach the version ConfigMap outside a cluster, so it must degrade
// to listing every tag instead of failing.
func TestGetAllTagsFallsBackToAllTagsOutsideCluster(t *testing.T) {
	serveGitHubTags(t, []string{"v0.4.9", "v0.4.8", "main-abc1234"})

	got, err := GetAllTags(context.Background())
	if err != nil {
		t.Fatalf("GetAllTags() error = %v, want nil", err)
	}

	want := []string{"v0.4.8", "v0.4.9", "main-abc1234"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetAllTags() = %v, want %v", got, want)
	}
}

// fakeSkopeo puts a stub skopeo on PATH that records its arguments and exits with
// the given code, so image verification is tested without a registry.
func fakeSkopeo(t *testing.T, exitCode int) (argsFile string) {
	t.Helper()

	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args.txt")
	script := fmt.Sprintf("#!/bin/sh\necho \"$@\" >> %s\nexit %d\n", argsFile, exitCode)

	if err := os.WriteFile(filepath.Join(dir, "skopeo"), []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake skopeo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return argsFile
}

func TestCheckImagesExist(t *testing.T) {
	argsFile := fakeSkopeo(t, 0)

	ok, err := CheckImagesExist(context.Background(), "v0.4.9")
	if err != nil {
		t.Fatalf("CheckImagesExist() error = %v, want nil", err)
	}
	if !ok {
		t.Error("CheckImagesExist() = false, want true when every image resolves")
	}

	recorded, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed to read recorded skopeo args: %v", err)
	}
	for _, image := range []string{
		"docker://quay.io/platform9/vjailbreak-ui:v0.4.9",
		"docker://quay.io/platform9/vjailbreak-controller:v0.4.9",
		"docker://quay.io/platform9/vjailbreak-vpwned:v0.4.9",
	} {
		if !strings.Contains(string(recorded), image) {
			t.Errorf("skopeo was not asked about %s; got:\n%s", image, recorded)
		}
	}
}

func TestCheckImagesExistMissingImage(t *testing.T) {
	fakeSkopeo(t, 1)

	ok, err := CheckImagesExist(context.Background(), "v9.9.9")
	if err == nil {
		t.Fatal("CheckImagesExist() error = nil, want an error when an image is missing")
	}
	if ok {
		t.Error("CheckImagesExist() = true, want false when an image is missing")
	}
	if !strings.Contains(err.Error(), "required image not found") {
		t.Errorf("error = %q, want it to say which image is missing", err)
	}
}

func TestCheckImagesExistStopsAtFirstMissingImage(t *testing.T) {
	argsFile := fakeSkopeo(t, 1)

	if _, err := CheckImagesExist(context.Background(), "v9.9.9"); err == nil {
		t.Fatal("CheckImagesExist() error = nil, want an error")
	}

	recorded, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed to read recorded skopeo args: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(recorded)), "\n") + 1; lines != 1 {
		t.Errorf("skopeo invocations = %d, want 1 (verification should stop at the first failure)", lines)
	}
}
