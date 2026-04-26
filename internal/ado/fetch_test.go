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

	tree, err := FetchTree(context.Background(), fetcher, 12345)
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

func TestFetchTreeStopsWhenNoParentExists(t *testing.T) {
	fetcher := fakeWorkItemFetcher{items: map[int]*WorkItem{
		12345: {ID: 12345, Fields: map[string]any{"System.WorkItemType": "Task"}},
	}}

	tree, err := FetchTree(context.Background(), fetcher, 12345)
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

	tree, err := FetchTree(context.Background(), fetcher, 1)
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

	_, err := FetchTree(context.Background(), fetcher, 1)
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
}

func (f fakeWorkItemFetcher) FetchWorkItem(ctx context.Context, id int) (*WorkItem, error) {
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
	return &WorkItem{
		ID:     id,
		Fields: map[string]any{"System.WorkItemType": workItemType},
		Relations: []Relation{{
			Rel: parentRelationType,
			URL: "https://dev.azure.com/org/_apis/wit/workItems/" + intString(parentID),
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
