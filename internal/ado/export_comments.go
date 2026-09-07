package ado

import (
	"html"
	"strings"
)

func renderWorkItemCommentsHTML(comments []WorkItemReadComment) string {
	var rendered strings.Builder
	rendered.WriteString("<h2>Discussion</h2>\n")
	if len(comments) == 0 {
		rendered.WriteString("<p>No comments were returned.</p>\n")
		return rendered.String()
	}
	for _, comment := range comments {
		rendered.WriteString("<article class=\"comment\">\n")
		if comment.Author != nil && comment.Author.DisplayName != nil {
			rendered.WriteString("<p><strong>Author:</strong> ")
			rendered.WriteString(html.EscapeString(*comment.Author.DisplayName))
			rendered.WriteString("</p>\n")
		}
		if comment.CreatedDate != nil {
			rendered.WriteString("<p><strong>Created:</strong> ")
			rendered.WriteString(html.EscapeString(*comment.CreatedDate))
			rendered.WriteString("</p>\n")
		}
		if comment.ModifiedDate != nil {
			rendered.WriteString("<p><strong>Modified:</strong> ")
			rendered.WriteString(html.EscapeString(*comment.ModifiedDate))
			rendered.WriteString("</p>\n")
		}
		rendered.WriteString("<div>")
		rendered.WriteString(html.EscapeString(comment.Text))
		rendered.WriteString("</div>\n</article>\n")
	}
	return rendered.String()
}
