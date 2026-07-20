package ado

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestFetchTreeWalksParentsUntilEpic(t *testing.T) {
	fetcher := fakeWorkItemFetcher{items: map[int]*WorkItem{
		12345: workItemWithParent(12345, "Task", 12001),
		12001: workItemWithParent(12001, "User Story", 10000),
		10000: {ID: 10000, Fields: map[string]any{"System.WorkItemType": "Epic", "System.Title": "Epic title"}},
	}}

	tree, err := FetchTree(context.Background(), fetcher, 12345, nil)
	if err != nil {
		t.Fatalf("FetchTree returned error: %v", err)
	}
	if ids := workItemIDs(tree.WorkItems); !reflect.DeepEqual(ids, []int{12345, 12001, 10000}) {
		t.Fatalf("tree IDs = %v, want input-to-Epic chain", ids)
	}
	if tree.EpicID != 10000 {
		t.Fatalf("EpicID = %d, want 10000", tree.EpicID)
	}
}

func TestFetchTreeIncludesRootParentChainAndDirectChildren(t *testing.T) {
	fetcher := fakeWorkItemFetcher{items: map[int]*WorkItem{
		12345: workItemWithParentAndChildren(12345, "User Story", 10000, 12346, 12347),
		10000: {ID: 10000, Fields: map[string]any{"System.WorkItemType": "Epic", "System.Title": "Epic title"}},
		12346: {ID: 12346, Fields: map[string]any{"System.WorkItemType": "Task", "System.Title": "First child"}},
		12347: {ID: 12347, Fields: map[string]any{"System.WorkItemType": "Task", "System.Title": "Second child"}},
	}}

	var progress []string
	tree, err := FetchTree(context.Background(), fetcher, 12345, func(message string) {
		progress = append(progress, message)
	})
	if err != nil {
		t.Fatalf("FetchTree returned error: %v", err)
	}
	if ids := workItemIDs(tree.WorkItems); !reflect.DeepEqual(ids, []int{12345, 10000, 12346, 12347}) {
		t.Fatalf("tree IDs = %v, want root, parent chain, then children", ids)
	}
	if tree.EpicID != 10000 {
		t.Fatalf("EpicID = %d, want 10000", tree.EpicID)
	}
	wantProgress := []string{
		"Fetching work item 12345 (parent chain)",
		"Fetching work item 10000 (parent chain)",
		"Fetching child work item 12346",
		"Fetching child work item 12347",
	}
	if !reflect.DeepEqual(progress, wantProgress) {
		t.Fatalf("progress = %q, want %q", progress, wantProgress)
	}
}

func TestFetchTreeSkipsChildRelationsAlreadyVisited(t *testing.T) {
	calls := map[int]int{}
	fetcher := fakeWorkItemFetcher{
		items: map[int]*WorkItem{
			1: workItemWithParentAndChildren(1, "User Story", 2, 2, 3, 3),
			2: {ID: 2, Fields: map[string]any{"System.WorkItemType": "Epic"}},
			3: workItemWithChild(3, "Task", 1),
		},
		calls: calls,
	}

	tree, err := FetchTree(context.Background(), fetcher, 1, nil)
	if err != nil {
		t.Fatalf("FetchTree returned error: %v", err)
	}
	if ids := workItemIDs(tree.WorkItems); !reflect.DeepEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("tree IDs = %v, want duplicates skipped", ids)
	}
	if calls[2] != 1 || calls[3] != 1 {
		t.Fatalf("fetch calls = %v, want each visited work item fetched once", calls)
	}
}

func TestFetchTreeReturnsMalformedChildRelationError(t *testing.T) {
	fetcher := fakeWorkItemFetcher{items: map[int]*WorkItem{
		1: {
			ID:     1,
			Fields: map[string]any{"System.WorkItemType": "Task"},
			Relations: []Relation{{
				Rel: childRelationType,
				URL: "https://dev.azure.com/org/_apis/wit/workItems/not-a-number",
			}},
		},
	}}

	_, err := FetchTree(context.Background(), fetcher, 1, nil)
	if err == nil {
		t.Fatal("FetchTree error = nil, want child relation parse error")
	}
	if !strings.Contains(err.Error(), "child relation URL") || !strings.Contains(err.Error(), "numeric") {
		t.Fatalf("error = %q, want child relation numeric ID context", err.Error())
	}
}

func TestFetchTreeReturnsChildFetchError(t *testing.T) {
	fetcher := fakeWorkItemFetcher{
		items: map[int]*WorkItem{1: workItemWithChild(1, "Task", 2)},
		errs:  map[int]error{2: errors.New("boom")},
	}

	_, err := FetchTree(context.Background(), fetcher, 1, nil)
	if err == nil {
		t.Fatal("FetchTree error = nil, want child fetch error")
	}
	if !strings.Contains(err.Error(), "fetching child work item 2") {
		t.Fatalf("error = %q, want child work item ID context", err.Error())
	}
}

func TestFetchTreeStopsWhenNoParentExists(t *testing.T) {
	fetcher := fakeWorkItemFetcher{items: map[int]*WorkItem{
		12345: {ID: 12345, Fields: map[string]any{"System.WorkItemType": "Task"}},
	}}

	tree, err := FetchTree(context.Background(), fetcher, 12345, nil)
	if err != nil {
		t.Fatalf("FetchTree returned error: %v", err)
	}
	if ids := workItemIDs(tree.WorkItems); !reflect.DeepEqual(ids, []int{12345}) {
		t.Fatalf("tree IDs = %v, want one item", ids)
	}
	if tree.EpicID != 0 {
		t.Fatalf("EpicID = %d, want 0", tree.EpicID)
	}
}

func TestFetchTreeStopsWhenParentAlreadyVisited(t *testing.T) {
	fetcher := fakeWorkItemFetcher{items: map[int]*WorkItem{
		1: workItemWithParent(1, "Task", 2),
		2: workItemWithParent(2, "User Story", 1),
	}}

	tree, err := FetchTree(context.Background(), fetcher, 1, nil)
	if err != nil {
		t.Fatalf("FetchTree returned error: %v", err)
	}
	if ids := workItemIDs(tree.WorkItems); !reflect.DeepEqual(ids, []int{1, 2}) {
		t.Fatalf("tree IDs = %v, want cycle stopped after two items", ids)
	}
}

func TestFetchTreeReturnsFetchError(t *testing.T) {
	fetcher := fakeWorkItemFetcher{
		items: map[int]*WorkItem{1: workItemWithParent(1, "Task", 2)},
		errs:  map[int]error{2: errors.New("boom")},
	}

	_, err := FetchTree(context.Background(), fetcher, 1, nil)
	if err == nil {
		t.Fatal("FetchTree error = nil, want error")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Fatalf("error = %q, want parent ID", err.Error())
	}
}

func TestParentIDFromRelationParsesLastNumericPathSegment(t *testing.T) {
	id, err := ParentIDFromRelation(Relation{URL: "https://dev.azure.com/org/_apis/wit/workItems/10000?api-version=7.1"})
	if err != nil {
		t.Fatalf("ParentIDFromRelation returned error: %v", err)
	}
	if id != 10000 {
		t.Fatalf("parent ID = %d, want 10000", id)
	}
}

func TestParentIDFromRelationReturnsErrorForMalformedURL(t *testing.T) {
	_, err := ParentIDFromRelation(Relation{URL: "https://dev.azure.com/org/_apis/wit/workItems/not-a-number"})
	if err == nil {
		t.Fatal("ParentIDFromRelation error = nil, want error")
	}
	if !strings.Contains(err.Error(), "numeric") {
		t.Fatalf("error = %q, want numeric ID message", err.Error())
	}
}

type fakeWorkItemFetcher struct {
	items map[int]*WorkItem
	errs  map[int]error
	calls map[int]int
}

func (f fakeWorkItemFetcher) FetchWorkItem(ctx context.Context, id int) (*WorkItem, error) {
	if f.calls != nil {
		f.calls[id]++
	}
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	item, ok := f.items[id]
	if !ok {
		return nil, errors.New("missing item")
	}
	return item, nil
}

func workItemWithParent(id int, workItemType string, parentID int) *WorkItem {
	return workItemWithParentAndChildren(id, workItemType, parentID)
}

func workItemWithParentAndChildren(id int, workItemType string, parentID int, childIDs ...int) *WorkItem {
	item := &WorkItem{
		ID:     id,
		Fields: map[string]any{"System.WorkItemType": workItemType},
		Relations: []Relation{{
			Rel: parentRelationType,
			URL: "https://dev.azure.com/org/_apis/wit/workItems/" + intString(parentID),
		}},
	}
	for _, childID := range childIDs {
		item.Relations = append(item.Relations, Relation{
			Rel: childRelationType,
			URL: "https://dev.azure.com/org/_apis/wit/workItems/" + intString(childID),
		})
	}
	return item
}

func workItemWithChild(id int, workItemType string, childID int) *WorkItem {
	return &WorkItem{
		ID:     id,
		Fields: map[string]any{"System.WorkItemType": workItemType},
		Relations: []Relation{{
			Rel: childRelationType,
			URL: "https://dev.azure.com/org/_apis/wit/workItems/" + intString(childID),
		}},
	}
}

func workItemIDs(items []WorkItem) []int {
	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func intString(v int) string {
	return strconv.Itoa(v)
}
