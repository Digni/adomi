package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const defaultAPIVersion = "7.1"
const maxPATRefBytes = 256

type Scope int

const (
	DefaultScope Scope = iota
	GlobalScope
)

type File struct {
	AzureDevOps AzureDevOpsConfig `yaml:"azureDevOps"`
}

type AzureDevOpsConfig struct {
	DefaultProfile string             `yaml:"defaultProfile"`
	Profiles       map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	Name         string `yaml:"-"`
	PATRef       string `yaml:"patRef"`
	BaseURL      string `yaml:"baseUrl"`
	Organization string `yaml:"organization"`
	Project      string `yaml:"project"`
	APIVersion   string `yaml:"apiVersion"`
	Proxy        string `yaml:"proxy"`
}

func (p Profile) CredentialRef() string {
	if ref := strings.TrimSpace(p.PATRef); ref != "" {
		return ref
	}
	return p.Name
}

type Loaded struct {
	Path    string
	Config  File
	Profile Profile
}

func Load(repoRoot, homeDir, requestedProfile string) (*Loaded, error) {
	return LoadWithScope(repoRoot, homeDir, requestedProfile, DefaultScope)
}

func LoadWithScope(repoRoot, homeDir, requestedProfile string, scope Scope) (*Loaded, error) {
	loaded, err := LoadAllWithScope(repoRoot, homeDir, scope)
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
	if profile.PATRef != "" {
		ref, err := NormalizePATRef(profile.PATRef)
		if err != nil {
			return nil, fmt.Errorf("Azure DevOps profile %q invalid patRef: %w", profile.Name, err)
		}
		profile.PATRef = ref
	}
	if err := validateProfile(profile); err != nil {
		return nil, err
	}

	loaded.Profile = profile
	return loaded, nil
}

func LoadAll(repoRoot, homeDir string) (*Loaded, error) {
	return LoadAllWithScope(repoRoot, homeDir, DefaultScope)
}

func LoadAllWithScope(repoRoot, homeDir string, scope Scope) (*Loaded, error) {
	path, err := findConfigPath(repoRoot, homeDir, scope)
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

func findConfigPath(repoRoot, homeDir string, scope Scope) (string, error) {
	homePath := GlobalConfigPath(homeDir)
	switch scope {
	case GlobalScope:
		if _, err := os.Stat(homePath); err == nil {
			return homePath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("checking global config %s: %w", homePath, err)
		}
		return "", fmt.Errorf("adomi global config not found at %s", homePath)
	case DefaultScope:
	default:
		return "", fmt.Errorf("unknown config scope %d", scope)
	}

	repoPath := filepath.Join(repoRoot, ".adomi", "config.yaml")
	if _, err := os.Stat(repoPath); err == nil {
		return repoPath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking repo config %s: %w", repoPath, err)
	}

	if _, err := os.Stat(homePath); err == nil {
		return homePath, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking home config %s: %w", homePath, err)
	}

	return "", fmt.Errorf("adomi config not found at %s or %s", repoPath, homePath)
}

func RepoConfigPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".adomi", "config.yaml")
}

func GlobalConfigPath(homeDir string) string {
	return filepath.Join(homeDir, ".config", "adomi", "config.yaml")
}

type InitTemplateValues struct {
	ProfileName  string
	BaseURL      string
	Organization string
	Project      string
	APIVersion   string
	Proxy        string
}

type AzureDevOpsRemoteInfo struct {
	Organization string
	Project      string
	Repository   string
	BaseURL      string
}

func RenderInitTemplate(values InitTemplateValues) string {
	values = defaultInitTemplateValues(values)
	values = sanitizeInitTemplateValues(values)
	lines := []string{
		"# azureDevOps:",
		"#   defaultProfile: " + values.ProfileName,
		"#",
		"#   profiles:",
		"#     " + values.ProfileName + ":",
		"#       patRef: \"\"",
		"#       baseUrl: " + values.BaseURL,
		"#       organization: " + values.Organization,
		"#       project: " + values.Project,
		"#       apiVersion: \"" + values.APIVersion + "\"",
		"#       proxy: \"" + values.Proxy + "\"",
		"",
	}
	return strings.Join(lines, "\n")
}

func DefaultInitTemplateValues() InitTemplateValues {
	return defaultInitTemplateValues(InitTemplateValues{})
}

func defaultInitTemplateValues(values InitTemplateValues) InitTemplateValues {
	if values.ProfileName == "" {
		values.ProfileName = "company-cloud"
	}
	if values.BaseURL == "" {
		values.BaseURL = "https://dev.azure.com/my-org"
	}
	if values.Organization == "" {
		values.Organization = "my-org"
	}
	if values.Project == "" {
		values.Project = "MyProject"
	}
	if values.APIVersion == "" {
		values.APIVersion = defaultAPIVersion
	}
	return values
}

func sanitizeInitTemplateValues(values InitTemplateValues) InitTemplateValues {
	values.ProfileName = sanitizeProfileName(singleLineTemplateValue(values.ProfileName))
	values.BaseURL = singleLineTemplateValue(values.BaseURL)
	values.Organization = singleLineTemplateValue(values.Organization)
	values.Project = singleLineTemplateValue(values.Project)
	values.APIVersion = singleLineTemplateValue(values.APIVersion)
	values.Proxy = singleLineTemplateValue(values.Proxy)
	return values
}

func singleLineTemplateValue(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r == '\n' || r == '\r' || unicode.IsControl(r) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteRune(r)
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func InitTemplateValuesFromRemotes(remotes []string) (InitTemplateValues, bool) {
	for _, remote := range remotes {
		if values, ok := InitTemplateValuesFromRemote(remote); ok {
			return values, true
		}
	}
	return InitTemplateValues{}, false
}

func InitTemplateValuesFromRemote(remote string) (InitTemplateValues, bool) {
	info, ok := AzureDevOpsRemoteInfoFromRemote(remote)
	if !ok {
		return InitTemplateValues{}, false
	}
	return initValues(info.Organization, info.Project), true
}

func AzureDevOpsRemoteInfoFromRemote(remote string) (AzureDevOpsRemoteInfo, bool) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return AzureDevOpsRemoteInfo{}, false
	}
	if strings.HasPrefix(remote, "git@ssh.dev.azure.com:v3/") {
		parts := splitAndUnescape(strings.Split(strings.TrimPrefix(remote, "git@ssh.dev.azure.com:v3/"), "/"))
		if len(parts) >= 3 {
			return remoteInfo(parts[0], parts[1], parts[2]), true
		}
		return AzureDevOpsRemoteInfo{}, false
	}

	parsed, err := url.Parse(remote)
	if err != nil || parsed.Host == "" {
		return AzureDevOpsRemoteInfo{}, false
	}
	host := strings.ToLower(parsed.Host)
	segments := splitAndUnescape(nonEmptyPathSegments(parsed.Path))
	switch {
	case host == "ssh.dev.azure.com" && len(segments) >= 4 && segments[0] == "v3":
		return remoteInfo(segments[1], segments[2], segments[3]), true
	case host == "dev.azure.com" && len(segments) >= 4 && segments[2] == "_git":
		return remoteInfo(segments[0], segments[1], segments[3]), true
	case strings.HasSuffix(host, ".visualstudio.com") && len(segments) >= 3 && segments[1] == "_git":
		org := strings.TrimSuffix(host, ".visualstudio.com")
		return remoteInfo(org, segments[0], segments[2]), true
	default:
		return AzureDevOpsRemoteInfo{}, false
	}
}

func initValues(org, project string) InitTemplateValues {
	org = strings.ToLower(org)
	return InitTemplateValues{
		ProfileName:  sanitizeProfileName(project),
		BaseURL:      "https://dev.azure.com/" + org,
		Organization: org,
		Project:      project,
		APIVersion:   defaultAPIVersion,
	}
}

func remoteInfo(org, project, repository string) AzureDevOpsRemoteInfo {
	org = strings.ToLower(org)
	return AzureDevOpsRemoteInfo{
		Organization: org,
		Project:      project,
		Repository:   repository,
		BaseURL:      "https://dev.azure.com/" + org,
	}
}

func splitAndUnescape(parts []string) []string {
	decoded := make([]string, 0, len(parts))
	for _, part := range parts {
		value, err := url.PathUnescape(part)
		if err != nil {
			value = part
		}
		decoded = append(decoded, value)
	}
	return decoded
}

func nonEmptyPathSegments(path string) []string {
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func sanitizeProfileName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "-")
	if value == "" {
		return "company-cloud"
	}
	return value
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

func NormalizePATRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("cannot be empty")
	}
	if len(ref) > maxPATRefBytes {
		return "", fmt.Errorf("cannot exceed %d bytes", maxPATRefBytes)
	}
	for _, r := range ref {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("cannot contain control characters")
		}
	}
	return ref, nil
}
