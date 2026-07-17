package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Digni/adomi/internal/config"
	"golang.org/x/term"
)

func (r Runner) runADOLogin(args []string, stdin io.Reader, stderr io.Writer) error {
	credentialArgs, err := parseCredentialArgs(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	credentialRef, err := resolveCredentialRef(deps, credentialArgs)
	if err != nil {
		return err
	}

	pat, err := deps.ReadSecret(fmt.Sprintf("Azure DevOps PAT for %s: ", credentialRef), stdin, stderr)
	if err != nil {
		return err
	}
	pat = strings.TrimRight(pat, "\r\n")
	if pat == "" {
		return fmt.Errorf("PAT cannot be empty")
	}

	if err := deps.PATStore.Set(credentialRef, pat); err != nil {
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
	credentialArgs, err := parseCredentialArgs(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	credentialRef, err := resolveCredentialRef(deps, credentialArgs)
	if err != nil {
		return err
	}
	return deps.PATStore.Delete(credentialRef)
}

func (r Runner) runADOProfilesList(args []string, stdout io.Writer) error {
	global, err := parseGlobalFlag(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()
	repoRoot := ""
	scope := config.DefaultScope
	homeDir, err := deps.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}
	if global {
		scope = config.GlobalScope
	} else {
		cwd, err := deps.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		repoRoot, err = deps.FindRepoRoot(cwd)
		if err != nil {
			return err
		}
	}
	loaded, err := deps.LoadAllConfig(repoRoot, homeDir, scope)
	if err != nil {
		return err
	}
	for _, profile := range loaded.ProfileNames() {
		fmt.Fprintln(stdout, profile)
	}
	return nil
}

func (r Runner) runADOConfigInit(args []string, stdout io.Writer) error {
	global, err := parseGlobalFlag(args)
	if err != nil {
		return err
	}
	deps := r.dependencies()

	var repoRoot string
	var homeDir string
	if global {
		homeDir, err = deps.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}
		if cwd, err := deps.Getwd(); err == nil {
			if resolved, err := deps.FindRepoRoot(cwd); err == nil {
				repoRoot = resolved
			}
		}
	} else {
		cwd, err := deps.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}
		repoRoot, err = deps.FindRepoRoot(cwd)
		if err != nil {
			return err
		}
	}

	targetPath := config.RepoConfigPath(repoRoot)
	if global {
		targetPath = config.GlobalConfigPath(homeDir)
	}

	values := config.DefaultInitTemplateValues()
	if repoRoot != "" {
		if remotes, err := deps.RemoteURLs(repoRoot); err == nil {
			if detected, ok := config.InitTemplateValuesFromRemotes(remotes); ok {
				values = detected
			}
		}
	}

	if err := writeNewFile(targetPath, []byte(config.RenderInitTemplate(values)), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(stdout, targetPath)
	return nil
}

func writeNewFile(path string, data []byte, perm os.FileMode) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config %s already exists", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("config %s already exists", path)
		}
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

func resolveCredentialRef(deps Dependencies, args credentialArgs) (string, error) {
	if args.patRef != "" {
		return args.patRef, nil
	}

	repoRoot := ""
	scope := config.DefaultScope
	var homeDir string
	var err error
	if args.global {
		scope = config.GlobalScope
		homeDir, err = deps.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
	} else {
		repoRoot, homeDir, err = resolveLocations(deps)
		if err != nil {
			return "", err
		}
	}

	loaded, err := deps.LoadConfig(repoRoot, homeDir, args.profile, scope)
	if err != nil {
		return "", err
	}
	return loaded.Profile.CredentialRef(), nil
}

type credentialArgs struct {
	profile string
	patRef  string
	global  bool
}

func parseCredentialArgs(args []string) (credentialArgs, error) {
	var parsed credentialArgs
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
				return credentialArgs{}, fmt.Errorf("--profile requires a value")
			}
			parsed.profile = args[i+1]
			i++
		case "--pat-ref":
			if i+1 >= len(args) {
				return credentialArgs{}, fmt.Errorf("--pat-ref requires a value")
			}
			ref, err := parsePATRefValue(args[i+1])
			if err != nil {
				return credentialArgs{}, err
			}
			parsed.patRef = ref
			i++
		case "--global":
			parsed.global = true
		default:
			return credentialArgs{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if parsed.global && parsed.profile == "" {
		return credentialArgs{}, fmt.Errorf("--global requires --profile")
	}
	if parsed.profile != "" && parsed.patRef != "" {
		return credentialArgs{}, fmt.Errorf("cannot use --profile with --pat-ref")
	}
	if parsed.profile == "" && parsed.patRef == "" {
		return credentialArgs{}, fmt.Errorf("--profile or --pat-ref is required")
	}
	return parsed, nil
}

func parsePATRefValue(value string) (string, error) {
	if value == "" || strings.HasPrefix(strings.TrimSpace(value), "-") {
		return "", fmt.Errorf("--pat-ref requires a value")
	}
	ref, err := config.NormalizePATRef(value)
	if err != nil {
		return "", fmt.Errorf("patRef %w", err)
	}
	return ref, nil
}

func parseGlobalFlag(args []string) (bool, error) {
	var global bool
	for _, arg := range args {
		switch arg {
		case "--global":
			global = true
		default:
			return false, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return global, nil
}
