package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

const defaultAPIVersion = "7.1"

type File struct {
	AzureDevOps AzureDevOpsConfig `yaml:"azureDevOps"`
}

type AzureDevOpsConfig struct {
	DefaultProfile string             `yaml:"defaultProfile"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Name         string `yaml:"-"`
	BaseURL      string `yaml:"baseUrl"`
	Organization string `yaml:"organization"`
	Project      string `yaml:"project"`
	APIVersion   string `yaml:"apiVersion"`
	Proxy        string `yaml:"proxy"`
}

type Loaded struct {
	Path    string
	Config  File
	Profile Profile
}

func Load(repoRoot, homeDir, requestedProfile string) (*Loaded, error) {
	loaded, err := LoadAll(repoRoot, homeDir)
	if err != nil {
		return nil, err
	}

	profileName := requestedProfile
	if profileName == "" {
		profileName = loaded.Config.AzureDevOps.DefaultProfile
	}
	if profileName == "" {
		return nil, fmt.Errorf("no Azure DevOps profile specified and defaultProfile is empty")
	}

	profile, ok := loaded.Config.AzureDevOps.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("Azure DevOps profile %q not found", profileName)
	}

	profile.Name = profileName
	if profile.APIVersion == "" {
		profile.APIVersion = defaultAPIVersion
	}
	if err := validateProfile(profile); err != nil {
		return nil, err
	}

	loaded.Profile = profile
	return loaded, nil
}

func LoadAll(repoRoot, homeDir string) (*Loaded, error) {
	path, err := findConfigPath(repoRoot, homeDir)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg File
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	if cfg.AzureDevOps.Profiles == nil {
		cfg.AzureDevOps.Profiles = map[string]Profile{}
	}

	return &Loaded{
		Path:   path,
		Config: cfg,
	}, nil
}

func (l *Loaded) ProfileNames() []string {
	names := make([]string, 0, len(l.Config.AzureDevOps.Profiles))
	for name := range l.Config.AzureDevOps.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func findConfigPath(repoRoot, homeDir string) (string, error) {
	repoPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if _, err := os.Stat(repoPath); err == nil {
		return repoPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking repo config %s: %w", repoPath, err)
	}

	homePath := filepath.Join(homeDir, ".config", "adomi", "config.yaml")
	if _, err := os.Stat(homePath); err == nil {
		return homePath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking home config %s: %w", homePath, err)
	}

	return "", fmt.Errorf("adomi config not found at %s or %s", repoPath, homePath)
}

func validateProfile(profile Profile) error {
	if profile.BaseURL == "" {
		return fmt.Errorf("Azure DevOps profile %q missing baseUrl", profile.Name)
	}
	if profile.Project == "" {
		return fmt.Errorf("Azure DevOps profile %q missing project", profile.Name)
	}
	return nil
}
