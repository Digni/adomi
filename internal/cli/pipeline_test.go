package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func TestADOPipelineListRejectsInvalidArgumentsBeforeDependencies(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "positional argument", args: []string{"ado", "pipeline", "list", "unexpected"}, want: "unknown argument"},
		{name: "missing profile value", args: []string{"ado", "pipeline", "list", "--profile"}, want: "--profile requires a value"},
		{name: "flag as profile value", args: []string{"ado", "pipeline", "list", "--profile", "--global"}, want: "--profile requires a value"},
		{name: "blank profile value", args: []string{"ado", "pipeline", "list", "--profile", " \t"}, want: "--profile requires a value"},
		{name: "repeated profile", args: []string{"ado", "pipeline", "list", "--profile", "one", "--profile", "two"}, want: "--profile cannot be repeated"},
		{name: "repeated global", args: []string{"ado", "pipeline", "list", "--global", "--global"}, want: "--global cannot be repeated"},
		{name: "unknown flag", args: []string{"ado", "pipeline", "list", "--unknown"}, want: "unknown argument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPipelineRunRejectsBeforeDependencies(t, tt.args, tt.want)
		})
	}
}

func TestADOPipelineGetRejectsInvalidArgumentsBeforeDependencies(t *testing.T) {
	veryLargeRunID := "9999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing run ID", args: []string{"ado", "pipeline", "get"}, want: "usage"},
		{name: "zero run ID", args: []string{"ado", "pipeline", "get", "0"}, want: "1..2147483647"},
		{name: "negative run ID", args: []string{"ado", "pipeline", "get", "-1"}, want: "1..2147483647"},
		{name: "non-decimal run ID", args: []string{"ado", "pipeline", "get", "abc"}, want: "1..2147483647"},
		{name: "fractional run ID", args: []string{"ado", "pipeline", "get", "1.5"}, want: "1..2147483647"},
		{name: "signed int32 overflow", args: []string{"ado", "pipeline", "get", "2147483648"}, want: "1..2147483647"},
		{name: "arbitrarily large run ID", args: []string{"ado", "pipeline", "get", veryLargeRunID}, want: "1..2147483647"},
		{name: "missing profile value", args: []string{"ado", "pipeline", "get", "1", "--profile"}, want: "--profile requires a value"},
		{name: "flag as profile value", args: []string{"ado", "pipeline", "get", "1", "--profile", "--global"}, want: "--profile requires a value"},
		{name: "blank profile value", args: []string{"ado", "pipeline", "get", "1", "--profile", " \t"}, want: "--profile requires a value"},
		{name: "repeated profile", args: []string{"ado", "pipeline", "get", "1", "--profile", "one", "--profile", "two"}, want: "--profile cannot be repeated"},
		{name: "repeated global", args: []string{"ado", "pipeline", "get", "1", "--global", "--global"}, want: "--global cannot be repeated"},
		{name: "extra positional", args: []string{"ado", "pipeline", "get", "1", "extra"}, want: "unknown argument"},
		{name: "unknown flag", args: []string{"ado", "pipeline", "get", "1", "--unknown"}, want: "unknown argument"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPipelineRunRejectsBeforeDependencies(t, tt.args, tt.want)
		})
	}
}

func TestADOPipelineRequiresRepositoryIncludingGlobalScope(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "list default scope", args: []string{"ado", "pipeline", "list"}},
		{name: "list global scope", args: []string{"ado", "pipeline", "list", "--global"}},
		{name: "get default scope", args: []string{"ado", "pipeline", "get", "1"}},
		{name: "get global scope", args: []string{"ado", "pipeline", "get", "2147483647", "--global"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loadConfigCalled := false
			credentialCalled := false
			httpClientCalled := false
			adoClientCalled := false
			repositoryErr := errors.New("pipeline repository required")
			runner := Runner{deps: Dependencies{
				PATStore: pipelinePATStoreFunc(func(string) (string, error) {
					credentialCalled = true
					return "", errors.New("should not read credential")
				}),
				Getwd:       func() (string, error) { return "/outside", nil },
				UserHomeDir: func() (string, error) { t.Fatal("UserHomeDir called after repository failure"); return "", nil },
				FindRepoRoot: func(string) (string, error) {
					return "", repositoryErr
				},
				LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
					loadConfigCalled = true
					return nil, errors.New("should not load config")
				},
				NewHTTPClient: func(string) (*http.Client, error) {
					httpClientCalled = true
					return nil, errors.New("should not create HTTP client")
				},
				NewADOClient: func(*http.Client, ado.ClientConfig) (ADOClient, error) {
					adoClientCalled = true
					return nil, errors.New("should not create ADO client")
				},
			}}
			var stdout bytes.Buffer

			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if !errors.Is(err, repositoryErr) {
				t.Fatalf("error = %v, want repository error", err)
			}
			if loadConfigCalled || credentialCalled || httpClientCalled || adoClientCalled {
				t.Fatalf("dependencies after repository failure: config=%t credential=%t HTTP=%t ADO=%t, want all false", loadConfigCalled, credentialCalled, httpClientCalled, adoClientCalled)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPipelineGetParsesSignedInt32RunIDBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		runID string
	}{
		{name: "minimum", runID: "1"},
		{name: "maximum", runID: "2147483647"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validationPassed := errors.New("run ID validation passed")
			runner := Runner{deps: Dependencies{
				Getwd: func() (string, error) { return "", validationPassed },
			}}
			var stdout bytes.Buffer

			err := runner.Run([]string{"ado", "pipeline", "get", tt.runID}, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if !errors.Is(err, validationPassed) {
				t.Fatalf("error = %v, want dependency sentinel proving run ID %s parsed", err, tt.runID)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPipelineListPropagatesProfileGlobalCredentialProxyAndClientConfig(t *testing.T) {
	httpClient := &http.Client{}
	client := &pipelineADOClient{}
	runner := Runner{deps: Dependencies{
		PATStore: pipelinePATStoreFunc(func(profile string) (string, error) {
			if profile != "shared-pipeline" {
				t.Fatalf("PATStore.Get profile = %q, want shared-pipeline", profile)
			}
			return "pipeline-pat", nil
		}),
		Getwd: func() (string, error) { return "/repo/subdir", nil },
		FindRepoRoot: func(start string) (string, error) {
			if start != "/repo/subdir" {
				t.Fatalf("FindRepoRoot start = %q, want /repo/subdir", start)
			}
			return "/repo", nil
		},
		UserHomeDir: func() (string, error) { return "/home/me", nil },
		LoadConfig: func(repoRoot, homeDir, requestedProfile string, scope config.Scope) (*config.Loaded, error) {
			if repoRoot != "/repo" || homeDir != "/home/me" || requestedProfile != "pipeline-profile" || scope != config.GlobalScope {
				t.Fatalf("LoadConfig args = %q %q %q %v, want repo/home/profile/global", repoRoot, homeDir, requestedProfile, scope)
			}
			return &config.Loaded{Profile: config.Profile{
				Name:       "pipeline-profile",
				PATRef:     "shared-pipeline",
				BaseURL:    "https://dev.azure.com/pipeline-org/collection",
				Project:    "Pipeline Project",
				APIVersion: "7.2-preview.1",
				Proxy:      "http://proxy.example:8080",
			}}, nil
		},
		NewHTTPClient: func(proxy string) (*http.Client, error) {
			if proxy != "http://proxy.example:8080" {
				t.Fatalf("proxy = %q, want configured proxy", proxy)
			}
			return httpClient, nil
		},
		NewADOClient: func(gotHTTPClient *http.Client, cfg ado.ClientConfig) (ADOClient, error) {
			if gotHTTPClient != httpClient {
				t.Fatal("NewADOClient received a different HTTP client")
			}
			want := ado.ClientConfig{
				BaseURL:    "https://dev.azure.com/pipeline-org/collection",
				Project:    "Pipeline Project",
				APIVersion: "7.2-preview.1",
				PAT:        "pipeline-pat",
			}
			if cfg != want {
				t.Fatalf("client config = %+v, want %+v", cfg, want)
			}
			return client, nil
		},
	}}
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "list", "--profile", "pipeline-profile", "--global"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.listCalls != 1 {
		t.Fatalf("ListInProgressPipelineRuns calls = %d, want 1", client.listCalls)
	}
	if stdout.String() != "{\"runs\":[]}\n" {
		t.Fatalf("stdout = %q, want non-null empty runs array", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestADOPipelineListPreservesRunOrderAndUsesNonNullRuns(t *testing.T) {
	t.Run("received order", func(t *testing.T) {
		client := &pipelineADOClient{runs: []ado.PipelineRun{
			{ID: 29, PipelineID: 3, PipelineName: "second queued", RunNumber: "20260715.2", Status: "inProgress"},
			{ID: 11, PipelineID: 2, PipelineName: "first queued", RunNumber: "20260715.1", Status: "inProgress"},
		}}
		runner := pipelineFakeRunner(t, client)
		var stdout bytes.Buffer

		err := runner.Run([]string{"ado", "pipeline", "list"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		want := "{\"runs\":[{\"id\":29,\"pipelineId\":3,\"pipelineName\":\"second queued\",\"runNumber\":\"20260715.2\",\"status\":\"inProgress\",\"result\":null,\"sourceBranch\":null,\"sourceVersion\":null,\"queueTime\":null,\"startTime\":null,\"finishTime\":null,\"webUrl\":null},{\"id\":11,\"pipelineId\":2,\"pipelineName\":\"first queued\",\"runNumber\":\"20260715.1\",\"status\":\"inProgress\",\"result\":null,\"sourceBranch\":null,\"sourceVersion\":null,\"queueTime\":null,\"startTime\":null,\"finishTime\":null,\"webUrl\":null}]}\n"
		if stdout.String() != want {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	})

	t.Run("nil reader slice", func(t *testing.T) {
		runner := pipelineFakeRunner(t, &pipelineADOClient{runs: nil})
		var stdout bytes.Buffer

		err := runner.Run([]string{"ado", "pipeline", "list"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
		if stdout.String() != "{\"runs\":[]}\n" {
			t.Fatalf("stdout = %q, want non-null empty runs array", stdout.String())
		}
	})
}

func TestADOPipelineGetPrintsExactNormalizedProjection(t *testing.T) {
	result := "partiallySucceeded"
	sourceBranch := "refs/heads/feature/pipeline-status"
	sourceVersion := "0123456789abcdef"
	webURL := "https://dev.azure.com/org/project/_build/results?buildId=42"
	queueTime := time.Date(2026, 7, 15, 8, 1, 2, 123456789, time.UTC)
	startTime := time.Date(2026, 7, 15, 8, 2, 0, 0, time.UTC)
	finishTime := time.Date(2026, 7, 15, 8, 3, 4, 500000000, time.UTC)
	client := &pipelineADOClient{run: &ado.PipelineRun{
		ID:            42,
		PipelineID:    7,
		PipelineName:  "Deploy API",
		RunNumber:     "20260715.4",
		Status:        "completed",
		Result:        &result,
		SourceBranch:  &sourceBranch,
		SourceVersion: &sourceVersion,
		QueueTime:     &queueTime,
		StartTime:     &startTime,
		FinishTime:    &finishTime,
		WebURL:        &webURL,
	}}
	runner := pipelineFakeRunner(t, client)
	var stdout, stderr bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "get", "42"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "{\"id\":42,\"pipelineId\":7,\"pipelineName\":\"Deploy API\",\"runNumber\":\"20260715.4\",\"status\":\"completed\",\"result\":\"partiallySucceeded\",\"sourceBranch\":\"refs/heads/feature/pipeline-status\",\"sourceVersion\":\"0123456789abcdef\",\"queueTime\":\"2026-07-15T08:01:02.123456789Z\",\"startTime\":\"2026-07-15T08:02:00Z\",\"finishTime\":\"2026-07-15T08:03:04.5Z\",\"webUrl\":\"https://dev.azure.com/org/project/_build/results?buildId=42\"}\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestADOPipelineGetPropagatesRunID(t *testing.T) {
	client := &pipelineADOClient{run: &ado.PipelineRun{ID: 2147483647, PipelineID: 1, PipelineName: "build", RunNumber: "1", Status: "inProgress"}}
	runner := pipelineFakeRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "get", "2147483647"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.gotRunID != 2147483647 {
		t.Fatalf("GetPipelineRun ID = %d, want 2147483647", client.gotRunID)
	}
	if stdout.String() == "" {
		t.Fatal("stdout empty, want projected run")
	}
}

func TestADOPipelineDependencyErrorsPassThroughWithoutStdout(t *testing.T) {
	dependencyErr := errors.New("arbitrary dependency marker: secret text remains unchanged")
	tests := []struct {
		name string
		args []string
		fail func(*Runner)
	}{
		{
			name: "repository",
			args: []string{"ado", "pipeline", "list"},
			fail: func(runner *Runner) {
				runner.deps.FindRepoRoot = func(string) (string, error) { return "", dependencyErr }
			},
		},
		{
			name: "config",
			args: []string{"ado", "pipeline", "list"},
			fail: func(runner *Runner) {
				runner.deps.LoadConfig = func(string, string, string, config.Scope) (*config.Loaded, error) { return nil, dependencyErr }
			},
		},
		{
			name: "keyring",
			args: []string{"ado", "pipeline", "list"},
			fail: func(runner *Runner) {
				runner.deps.PATStore = pipelinePATStoreFunc(func(string) (string, error) { return "", dependencyErr })
			},
		},
		{
			name: "HTTP client",
			args: []string{"ado", "pipeline", "list"},
			fail: func(runner *Runner) {
				runner.deps.NewHTTPClient = func(string) (*http.Client, error) { return nil, dependencyErr }
			},
		},
		{
			name: "ADO client",
			args: []string{"ado", "pipeline", "list"},
			fail: func(runner *Runner) {
				runner.deps.NewADOClient = func(*http.Client, ado.ClientConfig) (ADOClient, error) { return nil, dependencyErr }
			},
		},
		{
			name: "list reader",
			args: []string{"ado", "pipeline", "list"},
			fail: func(runner *Runner) {
				runner.deps.NewADOClient = func(*http.Client, ado.ClientConfig) (ADOClient, error) {
					return &pipelineADOClient{listErr: dependencyErr}, nil
				}
			},
		},
		{
			name: "get reader",
			args: []string{"ado", "pipeline", "get", "42"},
			fail: func(runner *Runner) {
				runner.deps.NewADOClient = func(*http.Client, ado.ClientConfig) (ADOClient, error) {
					return &pipelineADOClient{getErr: dependencyErr}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := pipelineFakeRunner(t, &pipelineADOClient{})
			tt.fail(&runner)
			var stdout bytes.Buffer

			err := runner.Run(tt.args, strings.NewReader(""), &stdout, &bytes.Buffer{})
			if err != dependencyErr {
				t.Fatalf("error = %v, want exact dependency error %v", err, dependencyErr)
			}
			if stdout.String() != "" {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

func TestADOPipelineEncodingFailureLeavesStdoutEmpty(t *testing.T) {
	invalidJSONTime := time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC)
	client := &pipelineADOClient{run: &ado.PipelineRun{
		ID:           42,
		PipelineID:   7,
		PipelineName: "Pipeline",
		RunNumber:    "42",
		Status:       "inProgress",
		QueueTime:    &invalidJSONTime,
	}}
	runner := pipelineFakeRunner(t, client)
	var stdout bytes.Buffer

	err := runner.Run([]string{"ado", "pipeline", "get", "42"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run error = nil, want JSON encoding error")
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func assertPipelineHelpContract(t *testing.T, stderr string) {
	t.Helper()
	for _, want := range []string{
		"inProgress",
		"YAML",
		"classic Build",
		"best-effort",
		"transactional snapshot",
		"1..2147483647",
		"--profile",
		"--global",
		"Git repository",
		"HTTPS",
		"loopback",
		"bypass configured proxies",
		"compact JSON",
		"result",
		"null",
		"vso.build",
		"polling",
		"stage",
		"classic Release",
		"mutation",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	}
}

func assertPipelineRunRejectsBeforeDependencies(t *testing.T, args []string, want string) {
	t.Helper()
	runner := pipelineFailOnDependencyRunner(t)
	var stdout bytes.Buffer

	err := runner.Run(args, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func pipelineRunHelp(t *testing.T, runner Runner, args []string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer

	err := runner.Run(args, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(%v) returned error: %v", args, err)
	}
	if stdout.String() != "" {
		t.Fatalf("Run(%v) stdout = %q, want empty", args, stdout.String())
	}
	if stderr.String() == "" {
		t.Fatalf("Run(%v) stderr empty, want help", args)
	}
	return stderr.String()
}

func pipelineFailOnDependencyRunner(t *testing.T) Runner {
	t.Helper()
	fail := func(name string) {
		t.Fatalf("%s called before pipeline argument validation/help completed", name)
	}
	return Runner{deps: Dependencies{
		PATStore: pipelinePATStoreFunc(func(string) (string, error) {
			fail("PATStore.Get")
			return "", nil
		}),
		ReadSecret: func(string, io.Reader, io.Writer) (string, error) {
			fail("ReadSecret")
			return "", nil
		},
		Getwd: func() (string, error) {
			fail("Getwd")
			return "", nil
		},
		UserHomeDir: func() (string, error) {
			fail("UserHomeDir")
			return "", nil
		},
		FindRepoRoot: func(string) (string, error) {
			fail("FindRepoRoot")
			return "", nil
		},
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			fail("LoadConfig")
			return nil, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) {
			fail("NewHTTPClient")
			return nil, nil
		},
		NewADOClient: func(*http.Client, ado.ClientConfig) (ADOClient, error) {
			fail("NewADOClient")
			return nil, nil
		},
	}}
}

func pipelineFakeRunner(t *testing.T, client ADOClient) Runner {
	t.Helper()
	return Runner{deps: Dependencies{
		PATStore:     pipelinePATStoreFunc(func(string) (string, error) { return "pipeline-pat", nil }),
		Getwd:        func() (string, error) { return "/repo/subdir", nil },
		UserHomeDir:  func() (string, error) { return "/home/me", nil },
		FindRepoRoot: func(string) (string, error) { return "/repo", nil },
		LoadConfig: func(string, string, string, config.Scope) (*config.Loaded, error) {
			return &config.Loaded{Profile: config.Profile{
				Name:       "pipeline-profile",
				PATRef:     "shared-pipeline",
				BaseURL:    "https://dev.azure.com/pipeline-org",
				Project:    "Pipeline Project",
				APIVersion: "7.1",
			}}, nil
		},
		NewHTTPClient: func(string) (*http.Client, error) { return http.DefaultClient, nil },
		NewADOClient:  func(*http.Client, ado.ClientConfig) (ADOClient, error) { return client, nil },
	}}
}

type pipelineADOClient struct {
	fakeADOClient
	runs      []ado.PipelineRun
	run       *ado.PipelineRun
	listErr   error
	getErr    error
	listCalls int
	gotRunID  int
}

func (f *pipelineADOClient) ListInProgressPipelineRuns(context.Context) ([]ado.PipelineRun, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.runs, nil
}

func (f *pipelineADOClient) GetPipelineRun(_ context.Context, id int) (*ado.PipelineRun, error) {
	f.gotRunID = id
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.run != nil {
		return f.run, nil
	}
	return &ado.PipelineRun{ID: id, PipelineID: 1, PipelineName: "pipeline", RunNumber: "1", Status: "inProgress"}, nil
}

type pipelinePATStoreFunc func(profile string) (string, error)

func (f pipelinePATStoreFunc) Get(profile string) (string, error) {
	return f(profile)
}

func (pipelinePATStoreFunc) Set(string, string) error {
	return errors.New("unexpected PATStore.Set call")
}

func (pipelinePATStoreFunc) Delete(string) error {
	return errors.New("unexpected PATStore.Delete call")
}
