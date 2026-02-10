package gitlab

// Entity types for GitLab provider.
const (
	TypeProject = "code.gitlab.project"
	TypeIssue   = "code.gitlab.issue"
)

// Relationship types for GitLab provider.
const (
	RelIssues = "issues"
	RelProject = "project"
)

// Attribute names for GitLab provider.
const (
	AttrVisibility      = "visibility"
	AttrArchived        = "archived"
	AttrState           = "state"
	AttrLabels          = "labels"
	AttrAssignee        = "assignee"
	AttrAuthor          = "author"
	AttrProjectName     = "project_name"
	AttrProjectPath     = "project_path"
	AttrWebURL          = "web_url"
	AttrCreatedAt       = "created_at"
	AttrUpdatedAt       = "updated_at"
	AttrMilestone       = "milestone"
)
