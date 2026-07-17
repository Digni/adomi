package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Digni/adomi/internal/ado"
	"github.com/Digni/adomi/internal/config"
)

func (r Runner) newADOMaintenanceClient(requestedProfile string, global bool) (ADOClient, error) {
	deps := r.dependencies()
	repoRoot, homeDir, err := resolveLocations(deps)
	if err != nil {
		return nil, err
	}
	scope := config.DefaultScope
	if global {
		scope = config.GlobalScope
	}
	loaded, err := deps.LoadConfig(repoRoot, homeDir, requestedProfile, scope)
	if err != nil {
		return nil, err
	}
	profileConfig := loaded.Profile

	pat, err := deps.PATStore.Get(profileConfig.CredentialRef())
	if err != nil {
		return nil, err
	}
	httpClient, err := deps.NewHTTPClient(profileConfig.Proxy)
	if err != nil {
		return nil, err
	}
	client, err := deps.NewADOClient(httpClient, ado.ClientConfig{
		BaseURL:    profileConfig.BaseURL,
		Project:    profileConfig.Project,
		APIVersion: profileConfig.APIVersion,
		PAT:        pat,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
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

func parseFlagValue(flagName string, args []string, index int) (string, error) {
	if index+1 >= len(args) || args[index+1] == "" || strings.HasPrefix(args[index+1], "-") {
		return "", fmt.Errorf("%s requires a value", flagName)
	}
	return args[index+1], nil
}

func validateSingleMessageSource(message, messageFile string) error {
	hasMessage := message != ""
	hasFile := messageFile != ""
	if hasMessage == hasFile {
		return fmt.Errorf("exactly one of --message or --message-file is required")
	}
	return nil
}

func readOptionalNonEmptyFile(flagName, path string) (string, bool, error) {
	if strings.TrimSpace(path) == "" {
		return "", false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", flagName, err)
	}
	if len(data) == 0 {
		return "", false, fmt.Errorf("%s cannot be empty", flagName)
	}
	return string(data), true, nil
}

func readOptionalNonBlankFile(flagName, path string) (string, bool, error) {
	content, provided, err := readOptionalNonEmptyFile(flagName, path)
	if err != nil || !provided {
		return content, provided, err
	}
	if strings.TrimSpace(content) == "" {
		return "", false, fmt.Errorf("%s cannot be empty", flagName)
	}
	return content, true, nil
}

func writeJSONLine(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
