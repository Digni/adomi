## Context

`adomi agent skill` currently models its target as a single `claude bool`. The command defaults to the user-level `.agents/skills` root, uses `--project` to switch the base to the repository root, and uses `--claude` to replace `.agents/skills` with `.claude/skills` (`internal/cli/agent.go:30-107`). All targets receive the same deterministic `adomi/SKILL.md`; provider selection does not affect the generated content (`internal/cli/agent.go:66-73`, `internal/cli/agent.go:110-239`). Existing installations are protected by confirmation or `--force`/`--yes`, with backup-and-restore replacement behavior (`internal/cli/agent.go:242-314`).

Current provider documentation lists the portable `.agents/skills/<name>/SKILL.md` layout for the requested clients:

- [Codex](https://developers.openai.com/codex/skills/) uses `.agents/skills` at repository and user scope.
- [OpenCode](https://opencode.ai/docs/skills/) discovers project and global `.agents/skills` alongside its native and Claude-compatible roots.
- [Pi](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/README.md#skills) discovers `.agents/skills` at project and user scope alongside Pi-native roots.
- [GitHub Copilot](https://docs.github.com/en/copilot/concepts/agents/about-agent-skills) supports `.agents/skills` for project and personal skills.
- [Cursor](https://cursor.com/docs/skills) supports `.agents/skills` for project and user skills alongside Cursor-native roots.

The change is standard risk: it adds a public CLI selector and validation to a known, bounded command, but it preserves existing paths, content, overwrite behavior, and rollback.

## Goals / Non-Goals

**Goals:**

- Make Codex, OpenCode, Pi, GitHub Copilot, Cursor, and Claude explicit supported values on `adomi agent skill`.
- Use the portable `.agents/skills` root for clients that support the Agent Skills standard instead of creating duplicate vendor-specific copies.
- Preserve global-by-default and repository-local `--project` scope behavior.
- Preserve `--claude` and all existing replacement and stream contracts.
- Fail before filesystem changes when provider selection is invalid or ambiguous.

**Non-Goals:**

- No provider executable detection, version probing, or network access.
- No installation into `.codex`, `.opencode`, `.pi`, `.copilot`, `.github`, or `.cursor` vendor-specific skill roots.
- No provider-specific generated `SKILL.md` variants or additional generated files.
- No multi-provider batch installation, symlink management, merge/update algorithm, or change to replacement semantics.
- No change to the Azure DevOps commands taught by the generated skill.

## Decisions

### Add one named provider selector over two physical target families

Add `--provider <name>` with exact lowercase values `codex`, `opencode`, `pi`, `github-copilot`, `cursor`, and `claude`. Omission keeps the current default shared target. Resolve providers as follows before applying global/project scope:

| Provider | Global root | Project root |
|---|---|---|
| `codex` | `~/.agents/skills` | `<repo>/.agents/skills` |
| `opencode` | `~/.agents/skills` | `<repo>/.agents/skills` |
| `pi` | `~/.agents/skills` | `<repo>/.agents/skills` |
| `github-copilot` | `~/.agents/skills` | `<repo>/.agents/skills` |
| `cursor` | `~/.agents/skills` | `<repo>/.agents/skills` |
| `claude` | `~/.claude/skills` | `<repo>/.claude/skills` |

The provider flag is an explicit compatibility and target-selection surface, not a request for distinct copies. A skill installed for any shared provider is already in the same location used by the other shared providers.

Alternative considered: write every provider's native directory. That duplicates identical content, creates divergent overwrite state, and ignores the standard directory all requested non-Claude clients already support.

Alternative considered: document provider compatibility without adding a flag. That would describe the current shared path accurately but would not give users an explicit, validated provider choice or a discoverable list in command help.

### Keep `--claude` as a quiet compatibility alias

`--provider claude` selects the existing Claude root. The existing `--claude` flag remains accepted and visible as a compatibility alias, without a deprecation warning that would add stderr output to otherwise successful commands. Combining `--claude` with any explicit `--provider` is rejected, even when the provider value is `claude`, so there is one unambiguous selector source.

Alternative considered: remove `--claude` or mark it deprecated through Cobra. Removal is breaking, while Cobra deprecation warnings would change established stream behavior.

### Validate provider selection before resolving scope or writing files

Normalize only surrounding CLI parsing state, not provider spelling: provider values remain exact lowercase identifiers. Reject an empty explicit value, unknown value, or `--provider`/`--claude` conflict before calling `UserHomeDir`, `Getwd`, `FindRepoRoot`, or any write helper. Existing `--global`/`--project` conflict validation remains unchanged.

Alternative considered: silently fall back to the shared root for unknown providers. That can report a successful installation for a client whose discovery behavior has not been verified.

### Keep one portable generated skill and existing write safety

Provider selection changes only root resolution. `adomiSkillName`, `generateSkillContent`, directory/file modes, stdout, prompts, confirmation, forced replacement, backup, and restoration behavior remain shared. This keeps the provider matrix testable without introducing content drift.

### Verify provider behavior as a target matrix

Use table-driven CLI tests for each provider at global and project scope, plus focused tests for default behavior, legacy `--claude`, unknown providers, selector conflicts, help text, empty stdout on validation failure, and existing replacement behavior. After focused tests pass, run formatting, the full Go test suite, the full Go build, documentation link/claim checks, and strict OpenSpec validation.

## Risks / Trade-offs

- [Risk] Users may assume each named provider writes a separate copy → Mitigation: help and documentation state that five providers share one `.agents/skills/adomi` installation and produce the same output path.
- [Risk] Provider discovery paths can change in future client releases → Mitigation: keep the mapping small, link current primary documentation, and avoid unverified native directories or runtime claims.
- [Risk] Running the command for a second shared provider encounters the existing target → Mitigation: document that the first shared installation already serves all shared providers and retain the current confirmation/force safeguards.
- [Risk] The term “provider” can also mean an LLM API provider → Mitigation: scope the flag only under `adomi agent skill` and describe its values as coding-agent clients in help and docs.

## Migration Plan

1. Add provider-matrix and validation tests while preserving existing shared and Claude test cases.
2. Add provider parsing and target-family resolution without changing generated content or write helpers.
3. Update CLI help, README, getting-started, and Azure DevOps reference documentation with the shared-provider matrix and legacy alias.
4. Run focused and full verification, then sync these delta specifications during archive.

Rollback removes the new provider selector and documentation while leaving the pre-existing default, `--project`, `--claude`, and installed skill directories unchanged. No data migration or cleanup is required.

## Open Questions

None. “GitHub” is treated as GitHub Copilot and “pie” as the Pi coding agent, matching the products that publish Agent Skills support.
