## 1. Command Surface and Argument Parsing

- [x] 1.1 Add a top-level `agent` command to the Cobra command tree without changing existing `ado` or `config` command behavior.
- [x] 1.2 Add an `agent skill` subcommand with usage for `[--claude] <path>` and help text describing the default shared-agent target and the Claude override.
- [x] 1.3 Implement parsing that accepts optional `--claude`, requires exactly one source path, rejects `--default`, and rejects unknown/extra arguments.

## 2. Skill Metadata and Path Resolution

- [x] 2.1 Resolve the provided source path, verify it exists, and reject non-directory paths with clear errors.
- [x] 2.2 Implement minimal `SKILL.md` front matter parsing for `name` and `description` when the source path already contains a skill file.
- [x] 2.3 Implement deterministic kebab-case name inference from the source directory basename when no source `SKILL.md` name is available.
- [x] 2.4 Validate the final skill name as a non-empty kebab-case identifier before computing the target path.

## 3. Skill File Creation

- [x] 3.1 Map omitted target flags to `~/.agents/skills/<skill-name>/` and `--claude` to `~/.claude/skills/<skill-name>/` using injectable home-directory/filesystem behavior for tests.
- [x] 3.2 Refuse to write when the target skill directory already exists and leave existing files unchanged.
- [x] 3.3 Create missing parent directories and the new skill directory on successful validation.
- [x] 3.4 Write `SKILL.md` with valid `name` and non-empty `description` front matter plus starter Markdown sections when generating a new skill.
- [x] 3.5 Preserve or install a valid source `SKILL.md` when the source path already provides one.

## 4. Output and Tests

- [x] 4.1 Print only the created skill directory path to stdout on success.
- [x] 4.2 Keep stdout empty for validation and creation errors.
- [x] 4.3 Add unit tests for default target creation, Claude target creation, unsupported `--default`, missing/unknown flags, missing or file source paths, existing target refusal, generated file content, and existing `SKILL.md` metadata usage.
- [x] 4.4 Update root/help tests so top-level help includes `agent` while existing `ado` and `config` expectations continue to pass.
- [x] 4.5 Run `go test ./...` and fix any regressions.
