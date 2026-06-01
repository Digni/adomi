package ado

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
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
	RepositoryID  string
	PullRequestID int
	Content       string
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
	CreatePullRequestThread(ctx context.Context, opts PullRequestThreadCreateOptions) (*PullRequestThread, error)
	CreatePullRequestThreadComment(ctx context.Context, opts PullRequestThreadCommentCreateOptions) (*PullRequestComment, error)
	UpdatePullRequestThread(ctx context.Context, opts PullRequestThreadUpdateOptions) (*PullRequestThread, error)
}

type PullRequestBundle struct {
	PullRequest *PullRequest        `json:"pullRequest"`
	Threads     []PullRequestThread `json:"threads"`
}

func FetchPullRequestBundle(ctx context.Context, fetcher PullRequestFetcher, id int) (*PullRequestBundle, error) {
	pr, err := fetcher.FetchPullRequest(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching pull request %d: %w", id, err)
	}
	if pr.Repository.ID == "" {
		return nil, fmt.Errorf("pull request %d response missing repository ID", id)
	}
	threads, err := fetcher.FetchPullRequestThreads(ctx, pr.Repository.ID, id)
	if err != nil {
		return nil, fmt.Errorf("fetching pull request %d threads: %w", id, err)
	}
	return &PullRequestBundle{PullRequest: pr, Threads: threads}, nil
}

type PullRequestExportOptions struct {
	RepoRoot  string
	Profile   string
	Project   string
	CreatedAt time.Time
}

type PullRequestIndex struct {
	Source        string                  `json:"source"`
	Profile       string                  `json:"profile"`
	Project       string                  `json:"project"`
	PullRequestID int                     `json:"pullRequestId"`
	Title         string                  `json:"title,omitempty"`
	Status        string                  `json:"status,omitempty"`
	IsDraft       bool                    `json:"isDraft,omitempty"`
	Repository    string                  `json:"repository,omitempty"`
	RepositoryID  string                  `json:"repositoryId,omitempty"`
	SourceRefName string                  `json:"sourceRefName,omitempty"`
	TargetRefName string                  `json:"targetRefName,omitempty"`
	CreatedAt     string                  `json:"createdAt"`
	ThreadCount   int                     `json:"threadCount"`
	CommentCount  int                     `json:"commentCount"`
	Threads       []PullRequestThreadInfo `json:"threads"`
}

type PullRequestThreadInfo struct {
	ID           int    `json:"id"`
	Status       string `json:"status,omitempty"`
	FilePath     string `json:"filePath,omitempty"`
	IsDeleted    bool   `json:"isDeleted,omitempty"`
	CommentCount int    `json:"commentCount"`
	Path         string `json:"path"`
}

func PullRequestOutputPath(repoRoot, profile, project string, pullRequestID int) string {
	return filepath.Join(repoRoot, ".adomi", "context", "pull-requests", strconv.Itoa(pullRequestID))
}

func ExportPullRequest(opts PullRequestExportOptions, bundle *PullRequestBundle) (string, error) {
	if bundle == nil || bundle.PullRequest == nil {
		return "", fmt.Errorf("pull request bundle is required")
	}
	if opts.CreatedAt.IsZero() {
		opts.CreatedAt = time.Now().UTC()
	}

	outputDir := PullRequestOutputPath(opts.RepoRoot, opts.Profile, opts.Project, bundle.PullRequest.ID)
	if err := os.RemoveAll(outputDir); err != nil {
		return "", fmt.Errorf("removing previous pull request export directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "threads"), 0o755); err != nil {
		return "", fmt.Errorf("creating pull request threads directory: %w", err)
	}

	threads := bundle.Threads
	sortedThreads := make([]PullRequestThread, len(threads))
	copy(sortedThreads, threads)
	sort.SliceStable(sortedThreads, func(i, j int) bool { return sortedThreads[i].ID < sortedThreads[j].ID })

	if err := writePrettyJSON(filepath.Join(outputDir, "pull-request.json"), bundle.PullRequest); err != nil {
		return "", err
	}
	if err := writePrettyJSON(filepath.Join(outputDir, "threads.json"), sortedThreads); err != nil {
		return "", err
	}
	for _, thread := range sortedThreads {
		threadPath := filepath.Join(outputDir, "threads", strconv.Itoa(thread.ID)+".json")
		if err := writePrettyJSON(threadPath, thread); err != nil {
			return "", err
		}
	}

	commentsMD := renderPullRequestCommentsMarkdown(bundle.PullRequest, sortedThreads)
	if err := os.WriteFile(filepath.Join(outputDir, "comments.md"), []byte(commentsMD), 0o644); err != nil {
		return "", fmt.Errorf("writing pull request comments markdown: %w", err)
	}

	if err := writePrettyJSON(filepath.Join(outputDir, "index.json"), newPullRequestIndex(opts, bundle.PullRequest, sortedThreads)); err != nil {
		return "", err
	}

	return outputDir, nil
}

func newPullRequestIndex(opts PullRequestExportOptions, pr *PullRequest, threads []PullRequestThread) PullRequestIndex {
	infos := make([]PullRequestThreadInfo, 0, len(threads))
	totalComments := 0
	for _, thread := range threads {
		filePath := ""
		if thread.ThreadContext != nil {
			filePath = thread.ThreadContext.FilePath
		}
		commentCount := nonDeletedCommentCount(thread.Comments)
		totalComments += commentCount
		infos = append(infos, PullRequestThreadInfo{
			ID:           thread.ID,
			Status:       thread.Status,
			FilePath:     filePath,
			IsDeleted:    thread.IsDeleted,
			CommentCount: commentCount,
			Path:         filepath.ToSlash(filepath.Join("threads", strconv.Itoa(thread.ID)+".json")),
		})
	}

	return PullRequestIndex{
		Source:        "azure-devops",
		Profile:       opts.Profile,
		Project:       opts.Project,
		PullRequestID: pr.ID,
		Title:         pr.Title,
		Status:        pr.Status,
		IsDraft:       pr.IsDraft,
		Repository:    pr.Repository.Name,
		RepositoryID:  pr.Repository.ID,
		SourceRefName: pr.SourceRefName,
		TargetRefName: pr.TargetRefName,
		CreatedAt:     opts.CreatedAt.UTC().Format(time.RFC3339),
		ThreadCount:   len(threads),
		CommentCount:  totalComments,
		Threads:       infos,
	}
}

func nonDeletedCommentCount(comments []PullRequestComment) int {
	count := 0
	for _, comment := range comments {
		if comment.IsDeleted {
			continue
		}
		count++
	}
	return count
}

func renderPullRequestCommentsMarkdown(pr *PullRequest, threads []PullRequestThread) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# PR %d: %s\n\n", pr.ID, escapeMarkdownText(pr.Title))
	if pr.Status != "" {
		fmt.Fprintf(&b, "- Status: %s\n", inlineCode(pr.Status))
	}
	if pr.SourceRefName != "" || pr.TargetRefName != "" {
		fmt.Fprintf(&b, "- Branches: %s -> %s\n", inlineCode(pr.SourceRefName), inlineCode(pr.TargetRefName))
	}
	if pr.Repository.Name != "" {
		fmt.Fprintf(&b, "- Repository: %s\n", inlineCode(pr.Repository.Name))
	}
	b.WriteString("\n")
	if len(threads) == 0 {
		b.WriteString("_No comment threads._\n")
		return b.String()
	}
	for _, thread := range threads {
		if thread.IsDeleted {
			continue
		}
		fmt.Fprintf(&b, "## Thread %d", thread.ID)
		if thread.Status != "" {
			fmt.Fprintf(&b, " (%s)", inlineCode(thread.Status))
		}
		b.WriteString("\n")
		if thread.ThreadContext != nil && thread.ThreadContext.FilePath != "" {
			fmt.Fprintf(&b, "- File: %s\n", inlineCode(thread.ThreadContext.FilePath))
		}
		b.WriteString("\n")
		for _, comment := range thread.Comments {
			if comment.IsDeleted {
				continue
			}
			author := authorDisplay(comment.Author)
			date := comment.PublishedDate
			fmt.Fprintf(&b, "**%s** at %s\n\n", escapeMarkdownText(author), inlineCode(date))
			b.WriteString(strings.TrimRight(comment.Content, "\n"))
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func escapeMarkdownText(value string) string {
	if value == "" {
		return value
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch r {
		case '\\', '`', '*', '_', '{', '}', '[', ']', '(', ')', '#', '+', '-', '.', '!', '|', '<', '>', '~':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func inlineCode(value string) string {
	if value == "" {
		return value
	}
	maxBackticks := 0
	current := 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > maxBackticks {
				maxBackticks = current
			}
		} else {
			current = 0
		}
	}
	delim := strings.Repeat("`", maxBackticks+1)
	pad := ""
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") {
		pad = " "
	}
	return delim + pad + value + pad + delim
}

func authorDisplay(author map[string]any) string {
	if author == nil {
		return "(unknown)"
	}
	if name, ok := author["displayName"].(string); ok && name != "" {
		return name
	}
	if name, ok := author["uniqueName"].(string); ok && name != "" {
		return name
	}
	return "(unknown)"
}
