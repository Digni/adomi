package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"golang.org/x/term"
)

func (r Runner) runADO(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: adomi ado <fetch|login|logout|profiles>")
	}

	switch args[0] {
	case "fetch":
		return r.runADOFetch(args[1:], stdout)
	case "login":
		return r.runADOLogin(args[1:], stdin, stderr)
	case "logout":
		return r.runADOLogout(args[1:])
	case "profiles":
		if len(args) == 2 && args[1] == "list" {
			return r.runADOProfilesList(stdout)
		}
		return fmt.Errorf("usage: adomi ado profiles list")
	default:
		return fmt.Errorf("unknown ado command %q", args[0])
	}
}

func (r Runner) runADOFetch(args []string, stdout io.Writer) error {
	workItemID, profile, err := parseFetchArgs(args)
	if err != nil {
		return err
	}

	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, profile)
	if err != nil {
		return err
	}
	profileConfig := loaded.Profile

	pat, err := deps.PATStore.Get(profileConfig.Name)
	if err != nil {
		return err
	}
	httpClient, err := deps.NewHTTPClient(profileConfig.Proxy)
	if err != nil {
		return err
	}
	client, err := deps.NewADOClient(httpClient, ado.ClientConfig{
		BaseURL:    profileConfig.BaseURL,
		Project:    profileConfig.Project,
		APIVersion: profileConfig.APIVersion,
		PAT:        pat,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	tree, err := deps.FetchTree(ctx, client, workItemID)
	if err != nil {
		return err
	}
	outputDir, err := deps.ExportContext(ctx, client, ado.ExportOptions{
		RepoRoot:  repoRoot,
		Profile:   profileConfig.Name,
		Project:   profileConfig.Project,
		CreatedAt: deps.Now(),
	}, tree)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, outputDir)
	return nil
}

func (r Runner) runADOLogin(args []string, stdin io.Reader, stderr io.Writer) error {
	profile, err := parseProfileFlag(args, true)
	if err != nil {
		return err
	}

	pat, err := r.dependencies().ReadSecret(fmt.Sprintf("Azure DevOps PAT for %s: ", profile), stdin, stderr)
	if err != nil {
		return err
	}
	pat = strings.TrimRight(pat, "\r\n")
	if pat == "" {
		return fmt.Errorf("PAT cannot be empty")
	}

	if err := r.dependencies().PATStore.Set(profile, pat); err != nil {
		return err
	}
	return nil
}

func readSecret(prompt string, stdin io.Reader, stderr io.Writer) (string, error) {
	fmt.Fprint(stderr, prompt)
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		data, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("reading PAT: %w", err)
		}
		return string(data), nil
	}

	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil {
		if err == io.EOF && len(line) > 0 {
			return line, nil
		}
		return "", fmt.Errorf("reading PAT: %w", err)
	}
	return line, nil
}

func (r Runner) runADOLogout(args []string) error {
	profile, err := parseProfileFlag(args, true)
	if err != nil {
		return err
	}
	return r.dependencies().PATStore.Delete(profile)
}

func (r Runner) runADOProfilesList(stdout io.Writer) error {
	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return err
	}
	loaded, err := deps.LoadAllConfig(repoRoot, homeDir)
	if err != nil {
		return err
	}
	for _, profile := range loaded.ProfileNames() {
		fmt.Fprintln(stdout, profile)
	}
	return nil
}

func resolveLocations(deps Dependencies) (string, string, error) {
	cwd, err := deps.Getwd()
	if err != nil {
		return "", "", fmt.Errorf("getting current directory: %w", err)
	}
	repoRoot, err := deps.FindRepoRoot(cwd)
	if err != nil {
		return "", "", err
	}
	homeDir, err := deps.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("getting home directory: %w", err)
	}
	return repoRoot, homeDir, nil
}

func parseProfileFlag(args []string, required bool) (string, error) {
	var profile string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" {
				return "", fmt.Errorf("--profile requires a value")
			}
			profile = args[i+1]
			i++
		default:
			return "", fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if required && profile == "" {
		return "", fmt.Errorf("--profile is required")
	}
	return profile, nil
}

func parseFetchArgs(args []string) (int, string, error) {
	if len(args) == 0 {
		return 0, "", fmt.Errorf("usage: adomi ado fetch <work-item-id> [--profile <profile-name>]")
	}
	workItemID, err := strconv.Atoi(args[0])
	if err != nil || workItemID <= 0 {
		return 0, "", fmt.Errorf("work item ID must be a positive integer")
	}
	profile, err := parseProfileFlag(args[1:], false)
	if err != nil {
		return 0, "", err
	}
	return workItemID, profile, nil
}
