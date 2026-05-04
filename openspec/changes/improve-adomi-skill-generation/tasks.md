## 1. Overwrite Confirmation

- [x] 1.1 Pass stdin and stderr through the full `newRootCommand` → `newAgentCommand` → `newAgentSkillCommand` chain so existing-target prompts use the `Runner.Run` stream boundary.
- [x] 1.2 Replace unconditional existing-target failure with a confirmation prompt written to stderr when the target skill directory already exists.
- [x] 1.3 Accept only explicit `y` or `yes` responses, case-insensitive, before replacing an existing skill.
- [x] 1.4 Add `--force` and `--yes` non-interactive replacement flags that skip the confirmation prompt for existing skills.
- [x] 1.5 Preserve existing skill contents and keep stdout empty when the user declines or provides any non-confirming response.
- [x] 1.6 Replace the existing skill directory and print only the target path to stdout when the user confirms or uses `--force`/`--yes`.
- [x] 1.7 Use a backup/restore replacement flow so the previous skill directory is restored where possible if confirmed replacement fails.
- [x] 1.8 Use unique temporary backup paths and report partial-cleanup failures instead of silently discarding them.

## 2. Generated Adomi Skill Content

- [x] 2.1 Always generate the skill with fixed name `adomi`, including when `--project` is used from another repository.
- [x] 2.2 Replace the generic generated skill template with deterministic `adomi` Azure DevOps guidance.
- [x] 2.3 Update the generated front matter description to mention Azure DevOps, ADO, work items, pull requests, and project context.
- [x] 2.4 Document `adomi ado fetch <work-item-id>` and `adomi ado pr <pull-request-id>` in the generated skill.
- [x] 2.5 Document configuration and credential commands: `adomi config init`, `adomi config init --global`, `adomi ado profiles list`, `adomi ado login`, and `adomi ado logout`.
- [x] 2.6 Add generated guidance that agents should inspect repo/global config and use repository or folder path structure as hints to resolve Azure DevOps project/profile context before asking the user.
- [x] 2.7 Add generated safety guidance that agents must never print, log, or expose PAT values.

## 3. Tests and Validation

- [x] 3.1 Update skill generation tests to assert the fixed `adomi` name and Azure DevOps-specific description and command guidance.
- [x] 3.2 Add tests for existing skill replacement confirmation, declined replacement, forced replacement, and prompt-on-stderr behavior.
- [x] 3.3 Add table-driven tests for all required generated command/guidance strings so future command drift is visible in one place.
- [x] 3.4 Ensure existing scope tests for global/project and Claude/default targets still pass.
- [x] 3.5 Add edge-case tests for global installs outside repositories, project repo-resolution errors, home-directory errors, bare `agent`, EOF decline, and short `y` confirmation.
- [x] 3.6 Run `go test ./...`.
- [x] 3.7 Run `openspec validate improve-adomi-skill-generation`.
