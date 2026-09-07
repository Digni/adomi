package cli

import (
	"context"
	"fmt"

	"github.com/Digni/adomi/internal/ado"
)

type workItemCommentReader interface {
	ListWorkItemComments(ctx context.Context, workItemID int) ([]ado.WorkItemReadComment, error)
}

func fetchWorkItemComments(ctx context.Context, reader workItemCommentReader, tree *ado.WorkItemTree, progress func(string)) (map[int][]ado.WorkItemReadComment, error) {
	if reader == nil {
		return nil, fmt.Errorf("work item comment reader is required")
	}
	if tree == nil {
		return nil, fmt.Errorf("work item tree is required")
	}
	comments := make(map[int][]ado.WorkItemReadComment, len(tree.WorkItems))
	for _, item := range tree.WorkItems {
		if _, seen := comments[item.ID]; seen {
			continue
		}
		if progress != nil {
			progress(fmt.Sprintf("Fetching comments for work item %d", item.ID))
		}
		itemComments, err := reader.ListWorkItemComments(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("fetching comments for work item %d: %w", item.ID, err)
		}
		if itemComments == nil {
			itemComments = []ado.WorkItemReadComment{}
		}
		comments[item.ID] = itemComments
		if progress != nil {
			progress(fmt.Sprintf("Fetched %d comments for work item %d", len(itemComments), item.ID))
		}
	}
	return comments, nil
}
