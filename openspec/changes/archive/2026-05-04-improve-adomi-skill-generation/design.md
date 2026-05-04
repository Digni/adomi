## Context

`adomi agent skill` currently creates a valid but generic `SKILL.md` named from the current repository. For the `adomi` CLI this produces a skill that says only to "work with adomi-specific context or workflows", which does not help agents discover or use `adomi` when the user asks for Azure DevOps or ADO work. The command also refuses existing skill directories, so users cannot conveniently regenerate an improved skill after the built-in template changes.

`adomi` already exposes Azure DevOps commands under `adomi ado`: work item context fetching, pull request comment fetching, credential login/logout, profile listing, and config initialization. The generated skill should encode this operational knowledge so an agent can retrieve Azure DevOps context before coding or reviewing. `--project` should only control where the `adomi` skill is installed; it does not mean the generated skill is named after the host project.

## Goals / Non-Goals

**Goals:**
- Always generate a useful skill named `adomi` instead of deriving the skill name from the current repository.
- Keep global/project as installation scopes: global writes to user skill roots and does not require being inside a repository; `--project` writes to the current repository's project-local skill root.
- Make the generated description trigger on Azure DevOps, ADO, work items, pull requests, and project context tasks.
- Document `adomi ado fetch`, `adomi ado pr`, `adomi ado login/logout`, `adomi ado profiles list`, and `adomi config init`/`--global` behavior in the skill.
- Tell agents to infer project/profile/config from repository config and from repository/folder path structure before asking the user.
- Tell agents never to print, log, or expose PAT values.
- Allow replacing an existing generated skill after an explicit confirmation prompt or via non-interactive `--force`/`--yes`.
- Preserve stdout/stderr conventions: prompts on stderr, successful created/replaced path on stdout.

**Non-Goals:**
- No automatic Azure DevOps network calls during skill creation.
- No LLM-generated skill content; the template should remain deterministic and testable.
- No merge/update algorithm for preserving user edits inside an existing skill.
- No new command namespace beyond `adomi agent skill`.
- No changes to Azure DevOps fetch/pr/login semantics.
- No automatic force behavior; replacement remains opt-in through stdin confirmation or explicit `--force`/`--yes`.

## Decisions

### Always generate the adomi skill

Replace the current generic template with static content tailored to `adomi` and use `adomi` as the generated skill name and heading for every scope. This avoids producing a misleading project-named skill whose body documents the `adomi` CLI. When users run `adomi agent skill --project` inside another repository, the command installs `.agents/skills/adomi/SKILL.md` (or `.claude/skills/adomi/SKILL.md` with `--claude`) in that project so agents working in that project can discover the `adomi` Azure DevOps helper locally.

Key generated sections:
- Purpose: use `adomi` to retrieve Azure DevOps context for agent work.
- When to use: Azure DevOps, ADO, work items, PR comments, backlog items, project context.
- Configuration discovery: inspect repo-local `.adomi/config.yaml`, global config, and folder/repo naming patterns before asking for project/profile.
- Commands: `adomi ado fetch <work-item-id>`, `adomi ado pr <pull-request-id>`, `adomi ado profiles list`, `adomi ado login`, `adomi ado logout`, `adomi config init [--global]`.
- Safety: never print, log, or expose PAT values.
- Workflow: fetch context, read generated files, use stdout path as the context directory.

Alternative considered: keep deriving the skill name from the current repo. That works for the `adomi` repo but is confusing when installing project-local guidance into another repo, because the content is about the `adomi` CLI rather than the host project.

### Prompt or force before replacing existing skills

Change existing-target behavior from unconditional failure to explicit replacement. If the target directory already exists, write a prompt to stderr and read one line from stdin. Accept only clear yes values (`y` or `yes`, case-insensitive) before replacing the existing target directory. Any other response cancels without changes. For scripts and non-interactive regeneration, support `--force` and `--yes` as aliases that replace without prompting.

Alternative considered: always overwrite. That is convenient for regeneration but risky because users may have edited the generated skill. Prompting by default keeps regeneration possible while preserving user control; `--force`/`--yes` handles intentional automation.

### Replace with a temporary backup for recoverability

For confirmed replacements, move the existing skill directory to a temporary backup path in the same parent directory, write the new deterministic `SKILL.md`, then remove the backup after success. If writing the new skill fails, restore the backup before returning the error where possible. This avoids the worst failure mode where a write error destroys the user's existing skill.

Alternative considered: remove and recreate directly. That is simpler but can destroy an existing skill if removal succeeds and the new write fails.

### Keep confirmation testable through existing Run boundary

Use the existing `stdin`, `stdout`, and `stderr` arguments passed to `Runner.Run`. Update the full command construction chain (`newRootCommand` → `newAgentCommand` → `newAgentSkillCommand`) so overwrite confirmation uses the same stream conventions as other interactive commands.

Alternative considered: direct `os.Stdin`/`os.Stderr`. That would make tests harder and break the project's programmatic execution boundary.

## Risks / Trade-offs

- [Risk] Replacing a user-edited skill may discard local edits → Mitigation: require explicit confirmation, make prompt wording clear that the existing skill will be replaced, and use backup/restore for write failures.
- [Risk] Static generated content may drift from actual command behavior → Mitigation: cover command references in tests and update the template alongside CLI changes.
- [Risk] Agents may over-infer project/profile from path structure incorrectly → Mitigation: instruct agents to inspect config first, use path structure as a hint, and ask the user when ambiguity remains.
- [Risk] Prompting can block non-interactive automation → Mitigation: existing target prompts only when replacement is needed and automation can pass `--force` or `--yes` to skip the prompt.

## Migration Plan

1. Update tests to expect fixed `adomi` skill naming, overwrite confirmation behavior, forced replacement behavior, and richer Azure DevOps skill content.
2. Pass stdin/stderr through the agent command construction chain and implement confirmation-based replacement.
3. Add `--force`/`--yes` replacement flags.
4. Replace the generic generated skill template with deterministic `adomi` usage guidance.
5. Implement backup/restore replacement behavior for confirmed or forced overwrites.
6. Run `go test ./...` and `openspec validate` for the change.
