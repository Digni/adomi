## Context

`adomi` is a Go CLI built on Cobra with a testable `Runner.Run(args, stdin, stdout, stderr)` boundary. Existing commands keep user-facing data on stdout and help/errors/prompts on stderr. Skill files in the looked-up Claude/shared-agent format are directory-based: a skill lives in `<skills-root>/<skill-name>/`, its entry file is `SKILL.md`, and that file starts with YAML front matter containing at least `name` and `description`, followed by Markdown instructions. Claude's canonical user skill root is `~/.claude/skills/`; the shared/default agent root for this change is `~/.agents/skills/`.

The requested command shape is now `adomi agent skill [--claude] .`: running without a target flag installs to the default shared-agent root, while `--claude` opts into Claude's user skill root.

## Goals / Non-Goals

**Goals:**
- Add a top-level `agent` namespace with an `agent skill` subcommand.
- Default to the shared target (`~/.agents/skills`) when no target flag is provided.
- Support `--claude` as the only target override, mapping to `~/.claude/skills`.
- Resolve a local source path such as `.` and use it to infer or read skill metadata.
- Create `<target-root>/<skill-name>/SKILL.md` with valid front matter and useful starter Markdown.
- Refuse unsafe writes, including invalid names and existing target directories.
- Preserve current stdout/stderr conventions and make the behavior unit-testable without touching real home directories.

**Non-Goals:**
- No `--default` flag; default behavior is selected by omitting `--claude`.
- No overwrite/update mode, delete mode, or sync mode.
- No interactive editor or LLM-generated skill content.
- No network calls or external package dependencies.
- No changes to Azure DevOps/config command behavior.

## Decisions

### Keep the command under a new `agent` namespace

Add `adomi agent skill` instead of placing the command under `ado` or `config`. This keeps agent tooling distinct from Azure DevOps context fetching and leaves room for future agent-related commands.

Alternative considered: `adomi skill`. A shorter top-level command is convenient, but `agent skill` is clearer once additional agent-oriented commands exist.

### Require a source path and make the shared target the default

The command form will be `adomi agent skill [--claude] <path>`. The path is resolved with `filepath.Abs`/`EvalSymlinks` where practical and must exist as a directory. Without a target flag, the command writes to `~/.agents/skills`. With `--claude`, it writes to `~/.claude/skills`.

Alternative considered: requiring either `--claude` or `--default`. The explicit target form is less surprising for first-time users, but the requested workflow is faster and cleaner when the shared agent directory is the common case.

### Infer metadata from existing skill files first, then from the source path

If `<path>/SKILL.md` exists and contains front matter with a valid `name`, use that name and preserve enough metadata/content to install a valid skill. If no `SKILL.md` exists, derive a kebab-case name from the path basename and generate a starter `SKILL.md` with:
- `name: <derived-name>`
- `description: "Use when working with <derived-name>."`
- Markdown sections for purpose, when to use, and process.

The generated content should be deterministic so tests can compare it directly.

Alternative considered: require `--name` and `--description`. That would avoid inference mistakes, but it makes the intended one-command `adomi agent skill .` flow less useful. Optional metadata flags can be added later if needed.

### Write through injectable filesystem/home dependencies

Extend `Dependencies` with narrowly scoped helpers needed by this command, such as `UserHomeDir`, path existence checks, directory creation, and file writing. Existing `UserHomeDir` already exists and can be reused. Keep the command implementation testable with temporary directories and dependency overrides rather than writing to the developer's actual home folder.

Alternative considered: direct `os.UserHomeDir`, `os.MkdirAll`, and `os.WriteFile` calls inside the command. That is simpler but harder to test safely and inconsistent with the existing dependency injection approach.

### Refuse overwrites

If `<target-root>/<skill-name>` already exists, return a non-zero error and leave it unchanged. This avoids accidentally replacing hand-authored skills. A future change can add `--force` or `--update` with explicit semantics.

Alternative considered: merge or overwrite existing skills. That creates risk around user-authored assets/scripts and is too broad for the initial creation command.

## Risks / Trade-offs

- [Risk] Path-derived skill names can be too generic, such as `repo` or `project` → Mitigation: validate and document the derived name; future metadata flags can improve this without changing the base command.
- [Risk] Existing `SKILL.md` parsing can become overly complex → Mitigation: implement minimal front matter parsing for `name`/`description` only and fall back to generation when no file exists.
- [Risk] Users may not realize omitted target flags install to `~/.agents` → Mitigation: define the default target explicitly in help, specs, and success output via the returned path.
- [Risk] Generated descriptions may be placeholders → Mitigation: produce a valid scaffold and keep content editable by the user.

## Migration Plan

1. Add tests for the new command surface, default target selection, Claude override selection, generated `SKILL.md`, invalid arguments, and no-overwrite behavior.
2. Implement command registration and helper functions behind the existing runner boundary.
3. Verify all existing CLI tests still pass.
4. No runtime migration or rollback is required; rollback is removing the new command code and tests before release.
