## Why

`adomi agent skill` already installs a portable Agent Skills `SKILL.md`, but its public surface distinguishes only a generic shared target and Claude. Users cannot explicitly select or discover compatibility with Codex, OpenCode, Pi, GitHub Copilot, or Cursor even though those clients support the shared `.agents/skills` convention.

## What Changes

- Add an optional `--provider <name>` selector to `adomi agent skill` for `codex`, `opencode`, `pi`, `github-copilot`, `cursor`, and `claude`.
- Keep the default global and project targets at `~/.agents/skills/adomi/` and `<repo>/.agents/skills/adomi/`; Codex, OpenCode, Pi, GitHub Copilot, and Cursor use that shared standards-based installation rather than receiving duplicate vendor-specific copies.
- Map `--provider claude` to the existing `.claude/skills` target and preserve `--claude` as a backward-compatible alias.
- Reject unknown providers and conflicting target selectors before changing the filesystem.
- Update command help, user documentation, and tests so supported providers, shared target behavior, scope selection, and replacement safeguards are explicit.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `agent-skill-creation`: Expand scoped skill installation requirements with named provider selection, shared-path mappings, compatibility behavior, and validation.
- `cli-command-surface`: Expose the provider selector and its compatibility rules on the public `adomi agent skill` command.
- `repository-documentation`: Document the supported provider names, their shared or Claude-specific skill roots, and backward-compatible invocation.

## Impact

- Affected code: `internal/cli/agent.go` and its tests.
- Affected public interface: `adomi agent skill` flags, help text, validation errors, and documented provider compatibility.
- Affected documentation/specification: README and agent-skill installation guidance plus the `agent-skill-creation`, `cli-command-surface`, and `repository-documentation` capabilities.
- Affected filesystem behavior: named shared providers continue to write one standard skill directory under `.agents/skills`; Claude continues to use `.claude/skills`.
- No new runtime dependencies, network calls, generated skill formats, or Azure DevOps behavior.
