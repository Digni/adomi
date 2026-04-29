## Why

Agent-specific skills are currently authored and copied by hand, which makes it easy to place files in the wrong home directory or miss the required `SKILL.md` metadata format. Adding a first-class `adomi agent skill` command lets users generate correctly shaped skills for Claude or the default shared agent directory from one CLI entry point.

## What Changes

- Add an `adomi agent skill [--claude] <path>` command for creating an agent skill from a local path such as `.`.
- Default to the shared/non-Claude target when no target flag is provided, writing under `~/.agents/skills/<skill-name>/`.
- Support a Claude target via `--claude`, writing under `~/.claude/skills/<skill-name>/`.
- Infer a kebab-case skill name from the source path basename when no existing `SKILL.md` supplies one.
- Generate a skill directory containing a `SKILL.md` file with valid YAML front matter (`name`, `description`) and a starter body based on the source path.
- Validate skill names as kebab-case and refuse to overwrite an existing installed skill unless a future change adds explicit overwrite behavior.
- Keep stdout reserved for the created skill path and report prompts/errors/help on stderr.

## Capabilities

### New Capabilities
- `agent-skill-creation`: Defines the command behavior for generating agent skill scaffolds in Claude and default shared-agent locations from a local path.

### Modified Capabilities
- `cli-command-surface`: Adds the public `adomi agent skill` command surface and stream behavior expectations.

## Impact

- Affected CLI code: `internal/cli/root.go`, new command parsing/runner code under `internal/cli`, and related tests.
- Affected filesystem behavior: resolves a local source path and creates directories and `SKILL.md` files under user home (`~/.claude/skills/` or `~/.agents/skills/`).
- No new external service dependencies are expected.
- Existing Azure DevOps and config commands should remain unchanged.
