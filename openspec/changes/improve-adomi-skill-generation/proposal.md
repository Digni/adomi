## Why

The generated `adomi` skill is currently a generic placeholder, so agents do not learn how to use `adomi` for Azure DevOps work. Skill creation also refuses existing targets, which makes it awkward to regenerate a newer version of the `adomi` skill after the built-in guidance improves.

## What Changes

- Always generate an `adomi` skill, regardless of the repository from which the command is run.
- Keep scope semantics: global installs to the user skill directory; `--project` installs the same `adomi` skill into the current repository's project-local skill directory.
- Generate an `adomi`-specific skill that teaches agents how to use `adomi` for Azure DevOps/ADO context work.
- Update the generated skill description so agents select it when asked to work with Azure DevOps, ADO work items, pull requests, or related project context.
- Document core `adomi` workflows in the generated skill, including `adomi ado fetch`, `adomi ado pr`, profile/PAT configuration, and global vs repository config behavior.
- Instruct agents to infer the Azure DevOps project/config from repository configuration and, when relevant, from the repository or folder path structure before asking the user.
- Allow regenerating an existing `adomi` skill by prompting the user for confirmation before replacing the installed skill.
- Preserve non-interactive safety by refusing to overwrite unless the user explicitly confirms through the CLI prompt or passes `--force`/`--yes`.

## Capabilities

### New Capabilities

### Modified Capabilities
- `agent-skill-creation`: Generated skill content becomes `adomi`-aware, skill naming is fixed to `adomi`, project scope installs that same skill into the current repo, and existing skill replacement is allowed after user confirmation.
- `cli-command-surface`: `adomi agent skill` may prompt on stderr before overwriting an existing skill while keeping successful output on stdout; `--force`/`--yes` replace without prompting.

## Impact

- Affected CLI code: `internal/cli/agent.go` and tests.
- Affected generated files: `SKILL.md` content for global/project and Claude/default skill targets.
- Affected UX: existing skill directories can be replaced after user confirmation or with `--force`/`--yes`; `--project` means project-local installation of the `adomi` skill, not deriving the skill name from the project.
- No external service calls or dependency changes are expected.
