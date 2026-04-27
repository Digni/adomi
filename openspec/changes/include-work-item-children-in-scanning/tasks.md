## 1. Relation Modeling

- [x] 1.1 Add a `System.LinkTypes.Hierarchy-Forward` child relation constant and a `WorkItem.ChildRelations()` helper in `internal/ado/models.go`.
- [x] 1.2 Refactor relation URL ID parsing so parent parsing remains compatible while child parsing can return clear child-specific errors.

## 2. Tree Fetching

- [x] 2.1 Update `FetchTree` to retain the originally requested/root work item and append direct children of that root item after the existing parent-chain traversal.
- [x] 2.2 Use a shared visited set for root, parent, and child fetches so duplicate or cyclic relations do not duplicate exported work items.
- [x] 2.3 Propagate child fetch and child relation parse failures with errors that include the relevant child work item/relation context.

## 3. Tests

- [x] 3.1 Add unit coverage showing `FetchTree` returns the requested item, parent chain, and direct children in deterministic order.
- [x] 3.2 Add unit coverage for duplicate/cyclic child relations so already visited items are not fetched/exported twice.
- [x] 3.3 Add unit coverage for malformed child relation URLs and child fetch failures.
- [x] 3.4 Update CLI or export integration tests that assert fetched/exported work item contents so child JSON/HTML/index/tree/attachment behavior is covered.

## 4. Verification

- [x] 4.1 Run `go test ./...` and ensure the full suite passes.
- [x] 4.2 Run `openspec validate include-work-item-children-in-scanning --strict` and resolve any proposal/spec/task validation issues.
