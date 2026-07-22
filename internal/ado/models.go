package ado

const (
	parentRelationType       = "System.LinkTypes.Hierarchy-Reverse"
	childRelationType        = "System.LinkTypes.Hierarchy-Forward"
	attachmentRelationType   = "AttachedFile"
	artifactLinkRelationType = "ArtifactLink"
)

type WorkItem struct {
	ID        int            `json:"id"`
	Rev       int            `json:"rev,omitempty"`
	URL       string         `json:"url,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
	Relations []Relation     `json:"relations,omitempty"`
}

type WorkItemCommentCreateOptions struct {
	WorkItemID int
	Text       string
}

type WorkItemComment struct {
	ID         int    `json:"id,omitempty"`
	CommentID  int    `json:"commentId,omitempty"`
	WorkItemID int    `json:"workItemId,omitempty"`
	URL        string `json:"url,omitempty"`
}

func (c WorkItemComment) CreatedID() int {
	if c.ID > 0 {
		return c.ID
	}
	return c.CommentID
}

type Relation struct {
	Rel        string         `json:"rel"`
	URL        string         `json:"url"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

func (w WorkItem) Type() string {
	return fieldString(w.Fields, "System.WorkItemType")
}

func (w WorkItem) Title() string {
	return fieldString(w.Fields, "System.Title")
}

func (w WorkItem) Description() string {
	return fieldString(w.Fields, "System.Description")
}

func (w WorkItem) ParentRelation() (Relation, bool) {
	for _, relation := range w.Relations {
		if relation.Rel == parentRelationType {
			return relation, true
		}
	}
	return Relation{}, false
}

func (w WorkItem) ChildRelations() []Relation {
	var children []Relation
	for _, relation := range w.Relations {
		if relation.Rel == childRelationType {
			children = append(children, relation)
		}
	}
	return children
}

func (w WorkItem) AttachmentRelations() []Relation {
	var attachments []Relation
	for _, relation := range w.Relations {
		if relation.Rel == attachmentRelationType {
			attachments = append(attachments, relation)
		}
	}
	return attachments
}

func (w WorkItem) HasArtifactLink(url string) bool {
	for _, relation := range w.Relations {
		if relation.Rel == artifactLinkRelationType && relation.URL == url {
			return true
		}
	}
	return false
}

func fieldString(fields map[string]any, key string) string {
	value, ok := fields[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
