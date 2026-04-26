package ado

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type WorkItemFetcher interface {
	FetchWorkItem(ctx context.Context, id int) (*WorkItem, error)
}

type WorkItemTree struct {
	RootID    int        `json:"rootWorkItemId"`
	EpicID    int        `json:"epicWorkItemId"`
	WorkItems []WorkItem `json:"workItems"`
}

func FetchTree(ctx context.Context, fetcher WorkItemFetcher, rootID int) (*WorkItemTree, error) {
	tree := &WorkItemTree{RootID: rootID}
	visited := map[int]bool{}
	currentID := rootID

	for {
		if visited[currentID] {
			break
		}
		visited[currentID] = true

		item, err := fetcher.FetchWorkItem(ctx, currentID)
		if err != nil {
			return nil, fmt.Errorf("fetching work item %d: %w", currentID, err)
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

	return tree, nil
}

func ParentIDFromRelation(relation Relation) (int, error) {
	parsed, err := url.Parse(relation.URL)
	if err != nil {
		return 0, fmt.Errorf("parsing parent relation URL %q: %w", relation.URL, err)
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

	return 0, fmt.Errorf("parent relation URL %q does not contain a numeric work item ID", relation.URL)
}
