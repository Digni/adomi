## Context

`adomi ado fetch <work-item-id>` currently builds a `WorkItemTree` by fetching the requested work item and walking its `System.LinkTypes.Hierarchy-Reverse` parent relations until an Epic or missing parent is reached. Export then serializes every item in `WorkItemTree.WorkItems`, renders matching HTML, downloads attachments per item, writes `tree.json`, and builds `index.json` from the same list.

Azure DevOps exposes direct child links on the same expanded work item response as `System.LinkTypes.Hierarchy-Forward` relations. Because the client already requests work items with `$expand=all`, child IDs can be discovered from the root item's existing relation payload without adding a new endpoint or command option.

## Goals / Non-Goals

**Goals:**

- Include direct children of the requested work item in fetched/exported context.
- Preserve the existing parent-chain-to-Epic behavior and CLI contract.
- Keep export generation centralized around `WorkItemTree.WorkItems` so JSON, HTML, attachments, `tree.json`, and `index.json` all include children consistently.
- Avoid duplicate exports if a relation cycle or overlapping relation points to an already fetched work item.
- Keep errors actionable when child relation IDs cannot be parsed or child work item fetches fail.

**Non-Goals:**

- Recursively fetching grandchildren or all descendants.
- Adding flags to opt in/out of child inclusion.
- Changing Azure DevOps configuration, credential handling, or output directory structure.
- Changing stdout/stderr stream behavior.

## Decisions

1. **Fetch direct root children after collecting the root and parent chain.**
   - Rationale: The root item is fetched first and contains the child relation list. Fetching children after the parent chain preserves the existing ordering prefix (`root`, parents up to Epic) for callers/tests that rely on it, while appending additional context.
   - Alternative considered: Fetch children immediately after the root before parents. This makes the hierarchy less readable and changes existing order more aggressively.

2. **Model child relations alongside parent and attachment relations.**
   - Add a `childRelationType` constant for `System.LinkTypes.Hierarchy-Forward` and a `WorkItem.ChildRelations()` helper mirroring `ParentRelation()` and `AttachmentRelations()`.
   - Rationale: Keeps relation filtering in the model layer and keeps traversal code focused on graph walking.
   - Alternative considered: Filter relation strings inline in `FetchTree`; rejected because relation-type knowledge is already centralized in `models.go`.

3. **Reuse a generic relation ID parser for both parent and child relations.**
   - Keep `ParentIDFromRelation` for compatibility/tests, but implement it through a shared helper such as `workItemIDFromRelation(kind, relation)` so child parse errors can mention "child relation URL" while parent errors remain stable.
   - Rationale: Avoids duplicating URL parsing logic and preserves clear error context.
   - Alternative considered: Rename `ParentIDFromRelation` to a generic public helper. This would require broader test and call-site churn without adding value.

4. **Use one visited set for parent and child fetches.**
   - Rationale: Prevents duplicate `WorkItems` entries and guards against malformed or cyclic hierarchy links.
   - Alternative considered: Separate parent and child visited sets; rejected because duplicate exports/index entries are not useful context.

## Risks / Trade-offs

- **Child relation fan-out increases network calls and export size** → Limit scope to direct children only and rely on existing per-work-item fetch behavior.
- **Malformed child relation URLs can fail the entire fetch** → Return a clear parse error naming the child relation URL, matching existing parent behavior.
- **Additional items alter exported `tree.json`/`index.json` contents** → Preserve root and parent order, append children deterministically in Azure DevOps relation order, and update tests to assert the new behavior.
- **Child attachments may increase runtime** → Reuse the existing attachment download loop so behavior is consistent with other fetched work items.

## Migration Plan

- Implement model/traversal changes and tests.
- No data migration is required. Existing export directories for the same root are already replaced on each fetch, so the next run will write the expanded context.
- Rollback is a code revert; no persisted schema or configuration changes are introduced.

## Open Questions

- None. The implementation will include direct children of the requested/root work item only.
