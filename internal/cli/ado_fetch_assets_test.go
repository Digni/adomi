package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Digni/adomi/internal/ado"
)

func TestADOFetchAssetsHTTPToFilesystemCountsFilesAndIncludesCommentsOnRequest(t *testing.T) {
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	for _, includeComments := range []bool{false, true} {
		t.Run(fmt.Sprintf("comments=%t", includeComments), func(t *testing.T) {
			var externalRequests, sharedRequests, commentRequests, commentImageRequests atomic.Int32
			external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				externalRequests.Add(1)
				_, _ = w.Write(imageBytes.Bytes())
			}))
			t.Cleanup(external.Close)
			var baseURL string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, password, authenticated := r.BasicAuth()
				if !authenticated || password != "secret-pat" {
					t.Error("missing configured PAT")
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				switch r.URL.Path {
				case "/org/MyProject/_apis/wit/workitems/1":
					_ = json.NewEncoder(w).Encode(ado.WorkItem{ID: 1, Fields: map[string]any{
						"System.WorkItemType": "Task",
						"System.Description":  `<img src="_apis/wit/attachments/shared?fileName=field.png&amp;download=true"><img src="` + external.URL + `/badge.png">`,
					}, Relations: []ado.Relation{{Rel: "AttachedFile", URL: baseURL + "/_apis/wit/attachments/shared?fileName=relation.png", Attributes: map[string]any{"name": "screenshot"}}}})
				case "/org/MyProject/_apis/wit/workitems/1/comments":
					commentRequests.Add(1)
					fmt.Fprint(w, `{"comments":[{"id":10,"workItemId":1,"format":"markdown","text":"![shared](/org/_apis/wit/attachments/shared?api-version=7.1) ![comment](/org/_apis/wit/attachments/comment-only)"}],"totalCount":1}`)
				case "/org/_apis/wit/attachments/shared", "/org/MyProject/_apis/wit/attachments/shared":
					sharedRequests.Add(1)
					_, _ = w.Write(imageBytes.Bytes())
				case "/org/_apis/wit/attachments/comment-only":
					commentImageRequests.Add(1)
					_, _ = w.Write(imageBytes.Bytes())
				default:
					t.Errorf("unexpected path %q", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)
			baseURL = server.URL + "/org"
			runner, outputDir := newAssetFetchTestRunner(t, baseURL)
			args := []string{"ado", "fetch", "1", "--json"}
			wantFiles, wantSources, wantCommentRequests := 1, 2, int32(0)
			if includeComments {
				args = append(args, "--include-comments")
				wantFiles, wantSources, wantCommentRequests = 2, 3, 1
			}
			var stdout, stderr bytes.Buffer
			if err := runner.Run(args, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if want := fmt.Sprintf("{\"path\":%q,\"workItems\":1,\"attachments\":%d}\n", outputDir, wantFiles); stdout.String() != want {
				t.Fatalf("stdout = %q, want %q", stdout.String(), want)
			}
			if sharedRequests.Load() != 1 || commentRequests.Load() != wantCommentRequests || commentImageRequests.Load() != wantCommentRequests || externalRequests.Load() != 0 {
				t.Fatalf("shared/comment/comment-image/external requests = %d/%d/%d/%d", sharedRequests.Load(), commentRequests.Load(), commentImageRequests.Load(), externalRequests.Load())
			}
			data, err := os.ReadFile(filepath.Join(outputDir, "assets", "1.json"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest ado.WorkItemAssets
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			if manifest.WorkItemID != 1 || len(manifest.Assets) != wantFiles+1 {
				t.Fatalf("manifest = %#v, want %d downloaded and one skipped asset", manifest, wantFiles)
			}
			for _, asset := range manifest.Assets {
				if asset.URL == external.URL+"/badge.png" {
					if asset.Status != "skipped" || asset.Path != "" || asset.Reason == "" {
						t.Fatalf("external asset = %#v, want explicit skipped reason and no file", asset)
					}
					continue
				}
				if asset.Status != "downloaded" || filepath.Ext(asset.Name) != ".png" {
					t.Fatalf("asset = %#v, want downloaded PNG", asset)
				}
				payload, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(asset.Path)))
				if err != nil || !bytes.Equal(payload, imageBytes.Bytes()) {
					t.Fatalf("asset %q does not preserve image bytes: %v", asset.Path, err)
				}
				if strings.Contains(asset.URL, "/shared") && len(asset.Sources) != wantSources {
					t.Fatalf("shared sources = %#v, want %d", asset.Sources, wantSources)
				}
			}
			files, err := os.ReadDir(filepath.Join(outputDir, "attachments", "1"))
			if err != nil || len(files) != wantFiles {
				t.Fatalf("attachment files = %d, error = %v; want %d", len(files), err, wantFiles)
			}
		})
	}
}

func TestADOFetchAssetsNonImageResponseFailsWithoutSuccessOutput(t *testing.T) {
	var imageRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/org/MyProject/_apis/wit/workitems/1":
			_ = json.NewEncoder(w).Encode(ado.WorkItem{ID: 1, Fields: map[string]any{
				"System.Description": `<img src="/org/_apis/wit/attachments/not-an-image">`,
			}})
		case "/org/_apis/wit/attachments/not-an-image":
			imageRequests.Add(1)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html>Sign in</html>")
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	runner, outputDir := newAssetFetchTestRunner(t, server.URL+"/org")
	var stdout bytes.Buffer
	err := runner.Run([]string{"ado", "fetch", "1", "--json"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "image") {
		t.Fatalf("Run error = %v, want invalid image failure", err)
	}
	if stdout.Len() != 0 || imageRequests.Load() != 1 {
		t.Fatalf("stdout/image requests = %q/%d, want empty/1", stdout.String(), imageRequests.Load())
	}
	for _, path := range []string{"index.json", "tree.json", "assets/1.json"} {
		if _, err := os.Stat(filepath.Join(outputDir, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("final artifact %q stat error = %v, want absent", path, err)
		}
	}
}

func newAssetFetchTestRunner(t *testing.T, baseURL string) (Runner, string) {
	t.Helper()
	repoRoot, homeDir := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	configuration := "azureDevOps:\n  defaultProfile: company-cloud\n  profiles:\n    company-cloud:\n      patRef: shared-ado\n      baseUrl: " + baseURL + "\n      project: MyProject\n"
	if err := os.WriteFile(configPath, []byte(configuration), 0o644); err != nil {
		t.Fatal(err)
	}
	return Runner{deps: Dependencies{
		PATStore:    &fakePATStore{values: map[string]string{"shared-ado": "secret-pat"}},
		Getwd:       func() (string, error) { return repoRoot, nil },
		UserHomeDir: func() (string, error) { return homeDir, nil },
	}}, filepath.Join(repoRoot, ".adomi", "context", "work-items", "1")
}
