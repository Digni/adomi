package ado

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ProgressFunc is an optional callback for progress messages during
// long-running operations. It is safe to call with nil (no-op).
type ProgressFunc func(msg string)

func (p ProgressFunc) report(msg string) {
	if p != nil {
		p(msg)
	}
}

type WorkItemFetcher interface {
	FetchWorkItem(ctx context.Context, id int) (*WorkItem, error)
}

type WorkItemMaintainer interface {
	CreateWorkItemComment(ctx context.Context, opts WorkItemCommentCreateOptions) (*WorkItemComment, error)
}

type WorkItemTree struct {
	RootID    int        `json:"rootWorkItemId"`
	EpicID    int        `json:"epicWorkItemId"`
	WorkItems []WorkItem `json:"workItems"`
}

func FetchTree(ctx context.Context, fetcher WorkItemFetcher, rootID int, progress ProgressFunc) (*WorkItemTree, error) {
	tree := &WorkItemTree{RootID: rootID}
	visited := map[int]bool{}
	currentID := rootID
	var rootItem WorkItem

	for {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		progress.report(fmt.Sprintf("Fetching work item %d (parent chain)", currentID))
		item, err := fetcher.FetchWorkItem(ctx, currentID)
		if err != nil {
			return nil, fmt.Errorf("fetching work item %d: %w", currentID, err)
		}
		if item.ID == rootID {
			rootItem = *item
		}
		tree.WorkItems = append(tree.WorkItems, *item)

		if item.Type() == "Epic" {
			tree.EpicID = item.ID
			break
		}

		parentRelation, ok := item.ParentRelation()
		if !ok {
			break
		}
		parentID, err := ParentIDFromRelation(parentRelation)
		if err != nil {
			return nil, err
		}
		if visited[parentID] {
			break
		}
		currentID = parentID
	}

	for _, childRelation := range rootItem.ChildRelations() {
		childID, err := ChildIDFromRelation(childRelation)
		if err != nil {
			return nil, err
		}
		if visited[childID] {
			continue
		}
		visited[childID] = true
		progress.report(fmt.Sprintf("Fetching child work item %d", childID))
		child, err := fetcher.FetchWorkItem(ctx, childID)
		if err != nil {
			return nil, fmt.Errorf("fetching child work item %d: %w", childID, err)
		}
		tree.WorkItems = append(tree.WorkItems, *child)
	}

	return tree, nil
}

func ParentIDFromRelation(relation Relation) (int, error) {
	return workItemIDFromRelation("parent", relation)
}

func ChildIDFromRelation(relation Relation) (int, error) {
	return workItemIDFromRelation("child", relation)
}

func workItemIDFromRelation(kind string, relation Relation) (int, error) {
	parsed, err := url.Parse(relation.URL)
	if err != nil {
		return 0, fmt.Errorf("parsing %s relation URL %q: %w", kind, relation.URL, err)
	}

	segments := strings.Split(parsed.Path, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment := segments[i]
		if segment == "" {
			continue
		}
		id, err := strconv.Atoi(segment)
		if err == nil {
			return id, nil
		}
	}

	return 0, fmt.Errorf("%s relation URL %q does not contain a numeric work item ID", kind, relation.URL)
}
