package ado

import (
	"context"
	"fmt"
)

type PullRequest struct {
	ID            int              `json:"pullRequestId"`
	Title         string           `json:"title,omitempty"`
	Description   string           `json:"description,omitempty"`
	Status        string           `json:"status,omitempty"`
	IsDraft       bool             `json:"isDraft,omitempty"`
	MergeStatus   string           `json:"mergeStatus,omitempty"`
	CreatedBy     map[string]any   `json:"createdBy,omitempty"`
	CreationDate  string           `json:"creationDate,omitempty"`
	ClosedDate    string           `json:"closedDate,omitempty"`
	SourceRefName string           `json:"sourceRefName,omitempty"`
	TargetRefName string           `json:"targetRefName,omitempty"`
	URL           string           `json:"url,omitempty"`
	Repository    PullRequestRepo  `json:"repository"`
	Reviewers     []map[string]any `json:"reviewers,omitempty"`
}

type PullRequestRepo struct {
	ID      string         `json:"id"`
	Name    string         `json:"name,omitempty"`
	URL     string         `json:"url,omitempty"`
	Project map[string]any `json:"project,omitempty"`
}

type PullRequestThread struct {
	ID              int                  `json:"id"`
	PublishedDate   string               `json:"publishedDate,omitempty"`
	LastUpdatedDate string               `json:"lastUpdatedDate,omitempty"`
	Status          string               `json:"status,omitempty"`
	IsDeleted       bool                 `json:"isDeleted,omitempty"`
	Comments        []PullRequestComment `json:"comments,omitempty"`
	ThreadContext   *ThreadContext       `json:"threadContext,omitempty"`
	Properties      map[string]any       `json:"properties,omitempty"`
}

type ThreadContext struct {
	FilePath       string        `json:"filePath,omitempty"`
	LeftFileStart  *FilePosition `json:"leftFileStart,omitempty"`
	LeftFileEnd    *FilePosition `json:"leftFileEnd,omitempty"`
	RightFileStart *FilePosition `json:"rightFileStart,omitempty"`
	RightFileEnd   *FilePosition `json:"rightFileEnd,omitempty"`
}

type FilePosition struct {
	Line   int `json:"line,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type PullRequestComment struct {
	ID                     int            `json:"id"`
	ParentCommentID        int            `json:"parentCommentId,omitempty"`
	Author                 map[string]any `json:"author,omitempty"`
	Content                string         `json:"content,omitempty"`
	PublishedDate          string         `json:"publishedDate,omitempty"`
	LastUpdatedDate        string         `json:"lastUpdatedDate,omitempty"`
	LastContentUpdatedDate string         `json:"lastContentUpdatedDate,omitempty"`
	CommentType            string         `json:"commentType,omitempty"`
	IsDeleted              bool           `json:"isDeleted,omitempty"`
}

type PullRequestsResponse struct {
	Count int           `json:"count"`
	Value []PullRequest `json:"value"`
}

type PullRequestThreadsResponse struct {
	Count int                 `json:"count"`
	Value []PullRequestThread `json:"value"`
}

type PullRequestIterationsResponse struct {
	Count int                    `json:"count"`
	Value []PullRequestIteration `json:"value"`
}

type PullRequestIteration struct {
	ID int `json:"id"`
}

type PullRequestIterationChangesResponse struct {
	ChangeEntries []PullRequestIterationChange `json:"changeEntries"`
	NextSkip      int                          `json:"nextSkip"`
	NextTop       int                          `json:"nextTop"`
}

type PullRequestIterationChange struct {
	ChangeTrackingID int                    `json:"changeTrackingId"`
	ChangeID         int                    `json:"changeId,omitempty"`
	ChangeType       string                 `json:"changeType,omitempty"`
	Item             PullRequestChangedItem `json:"item,omitempty"`
	OriginalPath     string                 `json:"originalPath,omitempty"`
}

type PullRequestChangedItem struct {
	Path string `json:"path,omitempty"`
}

type PullRequestListOptions struct {
	RepositoryID  string
	SourceRefName string
	TargetRefName string
	Status        string
}

type PullRequestCreateOptions struct {
	RepositoryID  string
	SourceRefName string
	TargetRefName string
	Title         string
	Description   string
}

type PullRequestUpdateOptions struct {
	RepositoryID  string
	PullRequestID int
	Title         *string
	Description   *string
}

type PullRequestThreadCreateOptions struct {
	RepositoryID             string
	PullRequestID            int
	Content                  string
	ThreadContext            *ThreadContext
	PullRequestThreadContext *PullRequestThreadContext
}

type PullRequestThreadContext struct {
	ChangeTrackingID int                      `json:"changeTrackingId,omitempty"`
	IterationContext *CommentIterationContext `json:"iterationContext,omitempty"`
}

type CommentIterationContext struct {
	FirstComparingIteration  int `json:"firstComparingIteration,omitempty"`
	SecondComparingIteration int `json:"secondComparingIteration,omitempty"`
}

type PullRequestIterationChangesOptions struct {
	RepositoryID  string
	PullRequestID int
	IterationID   int
	CompareTo     int
	Top           int
}

type PullRequestThreadCommentCreateOptions struct {
	RepositoryID  string
	PullRequestID int
	ThreadID      int
	Content       string
}

type PullRequestThreadUpdateOptions struct {
	RepositoryID  string
	PullRequestID int
	ThreadID      int
	Status        string
}

type PullRequestFetcher interface {
	FetchPullRequest(ctx context.Context, id int) (*PullRequest, error)
	FetchPullRequestThreads(ctx context.Context, repositoryID string, pullRequestID int) ([]PullRequestThread, error)
}

type PullRequestMaintainer interface {
	ListPullRequests(ctx context.Context, opts PullRequestListOptions) ([]PullRequest, error)
	CreatePullRequest(ctx context.Context, opts PullRequestCreateOptions) (*PullRequest, error)
	UpdatePullRequest(ctx context.Context, opts PullRequestUpdateOptions) (*PullRequest, error)
	ListPullRequestIterations(ctx context.Context, repositoryID string, pullRequestID int) ([]PullRequestIteration, error)
	ListPullRequestIterationChanges(ctx context.Context, opts PullRequestIterationChangesOptions) ([]PullRequestIterationChange, error)
	CreatePullRequestThread(ctx context.Context, opts PullRequestThreadCreateOptions) (*PullRequestThread, error)
	CreatePullRequestThreadComment(ctx context.Context, opts PullRequestThreadCommentCreateOptions) (*PullRequestComment, error)
	UpdatePullRequestThread(ctx context.Context, opts PullRequestThreadUpdateOptions) (*PullRequestThread, error)
}

type PullRequestBundle struct {
	PullRequest *PullRequest        `json:"pullRequest"`
	Threads     []PullRequestThread `json:"threads"`
}

func FetchPullRequestBundle(ctx context.Context, fetcher PullRequestFetcher, id int, progress ProgressFunc) (*PullRequestBundle, error) {
	progress.report(fmt.Sprintf("Fetching pull request %d", id))
	pr, err := fetcher.FetchPullRequest(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching pull request %d: %w", id, err)
	}
	if pr.Repository.ID == "" {
		return nil, fmt.Errorf("pull request %d response missing repository ID", id)
	}
	progress.report(fmt.Sprintf("Fetching pull request %d threads", id))
	threads, err := fetcher.FetchPullRequestThreads(ctx, pr.Repository.ID, id)
	if err != nil {
		return nil, fmt.Errorf("fetching pull request %d threads: %w", id, err)
	}
	progress.report(fmt.Sprintf("Fetched pull request %d with %d threads", id, len(threads)))
	return &PullRequestBundle{PullRequest: pr, Threads: threads}, nil
}
