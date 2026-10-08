package ado

import (
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"golang.org/x/net/html"
)

// AssetSource identifies the original content that refers to a local asset.
type AssetSource struct {
	Kind      string `json:"kind"`
	URL       string `json:"url"`
	Field     string `json:"field,omitempty"`
	CommentID int    `json:"commentId,omitempty"`
}

func workItemImageSources(item WorkItem, comments []WorkItemReadComment) []AssetSource {
	var sources []AssetSource
	fields := make([]string, 0, len(item.Fields))
	for name := range item.Fields {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	for _, name := range fields {
		value, ok := item.Fields[name].(string)
		if !ok {
			continue
		}
		for _, reference := range htmlImageURLs(value) {
			sources = append(sources, AssetSource{Kind: "field", Field: name, URL: reference})
		}
	}
	for _, comment := range comments {
		var references []string
		if comment.Format != nil && strings.EqualFold(*comment.Format, "html") {
			references = htmlImageURLs(comment.Text)
		} else {
			references = markdownImageURLs(comment.Text)
		}
		for _, reference := range references {
			sources = append(sources, AssetSource{Kind: "comment", CommentID: comment.ID, URL: reference})
		}
	}
	return sources
}

func htmlImageURLs(content string) []string {
	var references []string
	tokens := html.NewTokenizer(strings.NewReader(content))
	for {
		switch tokens.Next() {
		case html.ErrorToken:
			return references
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokens.Token()
			if token.Data != "img" {
				continue
			}
			for _, attr := range token.Attr {
				if attr.Key == "src" {
					references = append(references, attr.Val)
					break
				}
			}
		}
	}
}

func markdownImageURLs(content string) []string {
	var references []string
	source := []byte(content)
	document := goldmark.DefaultParser().Parse(text.NewReader(source))
	// Only AST image and raw-HTML nodes are inspected; code remains inert.
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := node.(type) {
		case *ast.Image:
			references = append(references, string(util.URLEscape(node.Destination, true)))
		case *ast.RawHTML:
			references = append(references, htmlImageURLs(string(node.Text(source)))...)
		case *ast.HTMLBlock:
			references = append(references, htmlImageURLs(string(node.Text(source)))...)
		}
		return ast.WalkContinue, nil
	})
	return references
}
