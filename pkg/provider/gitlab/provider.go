package gitlab

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"github.com/yourname/mifind/internal/provider"
	"github.com/yourname/mifind/internal/types"
)

// Provider implements the provider interface for GitLab.
// It connects to a GitLab server to search projects and issues.
type Provider struct {
	provider.BaseProvider
	client            *gitlab.Client
	baseURL           string
	configuredProjects map[string]bool // Map of project IDs that are configured for issue search
	searchIssues      bool
}

// NewProvider creates a new GitLab provider.
func NewProvider() *Provider {
	return &Provider{
		BaseProvider: *provider.NewBaseProvider(provider.ProviderMetadata{
			Name:        "gitlab",
			Description: "GitLab code repository",
			ConfigSchema: provider.AddStandardConfigFields(map[string]provider.ConfigField{
				"url": {
					Type:        "string",
					Required:    true,
					Description: "GitLab server URL (e.g., https://gitlab.com)",
				},
				"access_token": {
					Type:        "string",
					Required:    true,
					Description: "GitLab personal access token",
				},
				"search_issues": {
					Type:        "bool",
					Required:    false,
					Description: "Include issues in search (requires project configuration)",
				},
				"projects": {
					Type:        "[]string",
					Required:    false,
					Description: "List of project paths to include issues from (e.g., ['group/project', 'mygroup/myproject'])",
				},
			}),
		}),
		configuredProjects: make(map[string]bool),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "gitlab"
}

// Initialize sets up the GitLab provider with the given configuration.
func (p *Provider) Initialize(ctx context.Context, config map[string]any) error {
	// Get and set instance ID
	instanceID, ok := config["instance_id"].(string)
	if !ok || instanceID == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "instance_id is required", nil)
	}
	p.SetInstanceID(instanceID)

	// Get URL
	gitlabURL, ok := config["url"].(string)
	if !ok || gitlabURL == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "url is required", nil)
	}
	p.baseURL = gitlabURL

	// Get access token
	accessToken, ok := config["access_token"].(string)
	if !ok || accessToken == "" {
		return provider.NewProviderError(provider.ErrorTypeConfig, "access_token is required", nil)
	}

	// Get search_issues option (optional, defaults to false)
	searchIssues := false
	if si, ok := config["search_issues"].(bool); ok {
		searchIssues = si
	}
	p.searchIssues = searchIssues

	// Get projects list (optional)
	if projects, ok := config["projects"].([]interface{}); ok {
		for _, proj := range projects {
			if path, ok := proj.(string); ok {
				p.configuredProjects[path] = true
			}
		}
	}

	// Create GitLab client
	client, err := gitlab.NewClient(accessToken, gitlab.WithBaseURL(gitlabURL))
	if err != nil {
		return provider.NewProviderError(provider.ErrorTypeConfig, "failed to create GitLab client", err)
	}
	p.client = client

	// Test connection by fetching current user
	if _, _, err := p.client.Users.CurrentUser(); err != nil {
		return provider.NewProviderError(provider.ErrorTypeAuth, "failed to connect to GitLab", err)
	}

	return nil
}

// Discover performs a full discovery of all items.
// For GitLab, we return a subset of recent projects to avoid loading everything.
func (p *Provider) Discover(ctx context.Context) ([]types.Entity, error) {
	return p.discoverWithLimit(ctx, 50)
}

// DiscoverSince performs incremental discovery since the given timestamp.
func (p *Provider) DiscoverSince(ctx context.Context, since time.Time) ([]types.Entity, error) {
	// Get projects updated since the given time
	opts := &gitlab.ListProjectsOptions{
		LastActivityAfter: &since,
		OrderBy:           gitlab.Ptr("last_activity_at"),
		Sort:              gitlab.Ptr("desc"),
	}
	opts.ListOptions.Page = 1
	opts.ListOptions.PerPage = 50

	projects, _, err := p.client.Projects.ListProjects(opts)
	if err != nil {
		return nil, fmt.Errorf("list projects failed: %w", err)
	}

	entities := make([]types.Entity, 0, len(projects))
	for _, proj := range projects {
		entities = append(entities, p.projectToEntity(proj))
	}

	return entities, nil
}

// Hydrate retrieves full details of an entity by ID.
func (p *Provider) Hydrate(ctx context.Context, id string) (types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}

	resourceID := entityID.ResourceID()

	// Try to parse as "projectID" or "projectID/issueIID"
	projID, issueIID, hasIssue := parseResourceID(resourceID)

	if hasIssue {
		// Get issue - GitLab API expects int64 for IID
		issue, _, err := p.client.Issues.GetIssue(projID, int64(issueIID))
		if err != nil {
			return types.Entity{}, provider.ErrNotFound
		}
		// Get project for context
		project, _, err := p.client.Projects.GetProject(projID, nil)
		if err != nil {
			return p.issueToEntity(nil, issue), nil
		}
		return p.issueToEntity(project, issue), nil
	}

	// Get project
	project, _, err := p.client.Projects.GetProject(projID, nil)
	if err != nil {
		return types.Entity{}, provider.ErrNotFound
	}

	return p.projectToEntity(project), nil
}

// GetRelated retrieves entities related to an entity.
func (p *Provider) GetRelated(ctx context.Context, id string, relType string) ([]types.Entity, error) {
	entityID, err := provider.ParseEntityID(id)
	if err != nil {
		return nil, err
	}

	resourceID := entityID.ResourceID()
	projID, _, _ := parseResourceID(resourceID)

	switch relType {
	case RelIssues:
		// Get issues for a project
		if !p.shouldSearchIssues(projID) {
			return nil, provider.ErrNotFound
		}

		opts := &gitlab.ListProjectIssuesOptions{}
		opts.ListOptions.PerPage = 100

		issues, _, err := p.client.Issues.ListProjectIssues(projID, opts)
		if err != nil {
			return nil, err
		}

		// Get project for context
		project, _, _ := p.client.Projects.GetProject(projID, nil)

		entities := make([]types.Entity, 0, len(issues))
		for _, issue := range issues {
			entities = append(entities, p.issueToEntity(project, issue))
		}
		return entities, nil

	case RelProject:
		// Get project for an issue
		project, _, err := p.client.Projects.GetProject(projID, nil)
		if err != nil {
			return nil, err
		}
		return []types.Entity{p.projectToEntity(project)}, nil

	default:
		return nil, provider.ErrNotFound
	}
}

// Search performs a search query on this provider.
func (p *Provider) Search(ctx context.Context, query provider.SearchQuery) ([]types.Entity, error) {
	var entities []types.Entity

	// Search projects
	projectOpts := &gitlab.ListProjectsOptions{
		Search: gitlab.Ptr(query.Query),
	}
	if query.Limit > 0 {
		projectOpts.ListOptions.PerPage = int64(query.Limit)
	}

	// Apply filters
	if archived, ok := query.Filters[AttrArchived].(bool); ok {
		projectOpts.Archived = &archived
	}
	if visibility, ok := query.Filters[AttrVisibility].(string); ok {
		v := gitlab.VisibilityValue(visibility)
		projectOpts.Visibility = &v
	}

	projects, _, err := p.client.Projects.ListProjects(projectOpts)
	if err != nil {
		return nil, fmt.Errorf("list projects failed: %w", err)
	}

	for _, proj := range projects {
		entities = append(entities, p.projectToEntity(proj))
	}

	// Search issues if enabled and we have configured projects
	if p.searchIssues && (query.Type == TypeIssue || query.Type == "") {
		for projPath := range p.configuredProjects {
			issueOpts := &gitlab.ListProjectIssuesOptions{
				Search: gitlab.Ptr(query.Query),
			}
			issueOpts.ListOptions.PerPage = 50

			// Apply filters
			if state, ok := query.Filters[AttrState].(string); ok {
				issueOpts.State = &state
			}
			if labels, ok := query.Filters[AttrLabels].([]string); ok && len(labels) > 0 {
				labelOpts := gitlab.LabelOptions(labels)
				issueOpts.Labels = &labelOpts
			}

			issues, _, err := p.client.Issues.ListProjectIssues(projPath, issueOpts)
			if err == nil {
				project, _, _ := p.client.Projects.GetProject(projPath, nil)
				for _, issue := range issues {
					entities = append(entities, p.issueToEntity(project, issue))
				}
			}
		}
	}

	return entities, nil
}

// FilterCapabilities returns the filter capabilities for GitLab.
func (p *Provider) FilterCapabilities(ctx context.Context) (map[string]provider.FilterCapability, error) {
	caps := map[string]provider.FilterCapability{
		AttrVisibility: {
			Type:        types.AttributeTypeString,
			SupportsEq:  true,
			Description: "Filter by project visibility",
			Options: []provider.FilterOption{
				{Value: "public", Label: "Public"},
				{Value: "private", Label: "Private"},
				{Value: "internal", Label: "Internal"},
			},
		},
		AttrArchived: {
			Type:        types.AttributeTypeBool,
			SupportsEq:  true,
			Description: "Filter by archived status",
		},
	}

	// Add issue filters if searching issues is enabled
	if p.searchIssues {
		caps[AttrState] = provider.FilterCapability{
			Type:        types.AttributeTypeString,
			SupportsEq:  true,
			Description: "Filter by issue state",
			Options: []provider.FilterOption{
				{Value: "opened", Label: "Open"},
				{Value: "closed", Label: "Closed"},
			},
		}
		caps[AttrLabels] = provider.FilterCapability{
			Type:        types.AttributeTypeString,
			SupportsEq:  true,
			Description: "Filter by issue labels",
		}
	}

	return caps, nil
}

// FilterValues returns available filter values for the given filter name.
func (p *Provider) FilterValues(ctx context.Context, filterName string) ([]provider.FilterOption, error) {
	switch filterName {
	case AttrLabels:
		// Collect all labels from configured projects
		labelSet := make(map[string]bool)
		for projPath := range p.configuredProjects {
			opts := &gitlab.ListLabelsOptions{}
			opts.ListOptions.PerPage = 100
			labels, _, err := p.client.Labels.ListLabels(projPath, opts)
			if err == nil {
				for _, label := range labels {
					if label.Name != "" {
						labelSet[label.Name] = true
					}
				}
			}
		}

		options := make([]provider.FilterOption, 0, len(labelSet))
		for name := range labelSet {
			options = append(options, provider.FilterOption{
				Value: name,
				Label: name,
			})
		}
		return options, nil

	default:
		return nil, nil
	}
}

// Shutdown gracefully shuts down the provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	return nil
}

// projectToEntity converts a GitLab Project to a types.Entity.
func (p *Provider) projectToEntity(project *gitlab.Project) types.Entity {
	entityID := p.BuildEntityID(formatProjectID(project)).String()

	entity := types.NewEntity(entityID, TypeProject, p.Name(), project.Name)

	if project.Description != "" {
		entity.Description = project.Description
	}

	// Add timestamp
	if project.LastActivityAt != nil {
		entity.Timestamp = *project.LastActivityAt
	}

	// Add attributes - these are value types, not pointers
	entity.AddAttribute(AttrVisibility, string(project.Visibility))
	entity.AddAttribute(AttrArchived, project.Archived)
	entity.AddAttribute(AttrWebURL, project.WebURL)
	entity.AddAttribute(AttrProjectPath, project.PathWithNamespace)

	if project.CreatedAt != nil {
		entity.AddAttribute(AttrCreatedAt, project.CreatedAt.Format(time.RFC3339))
	}
	if project.UpdatedAt != nil {
		entity.AddAttribute(AttrUpdatedAt, project.UpdatedAt.Format(time.RFC3339))
	}

	// Add search tokens
	entity.AddSearchToken(project.Name)
	entity.AddSearchToken(project.PathWithNamespace)
	if project.Description != "" {
		entity.AddSearchToken(project.Description)
	}

	// Add relationship to issues if configured
	if p.shouldSearchIssues(formatProjectID(project)) {
		entity.AddRelationship(RelIssues, "")
	}

	return entity
}

// issueToEntity converts a GitLab Issue to a types.Entity.
func (p *Provider) issueToEntity(project *gitlab.Project, issue *gitlab.Issue) types.Entity {
	projID := "unknown"
	if project != nil {
		projID = formatProjectID(project)
	}
	entityID := p.BuildEntityID(fmt.Sprintf("%s/%d", projID, issue.IID)).String()

	title := issue.Title
	if title == "" {
		title = fmt.Sprintf("Issue #%d", issue.IID)
	}

	entity := types.NewEntity(entityID, TypeIssue, p.Name(), title)

	if issue.Description != "" {
		entity.Description = issue.Description
	}

	// Add timestamp - UpdateAt is a pointer
	if issue.UpdatedAt != nil {
		entity.Timestamp = *issue.UpdatedAt
	}

	// Add attributes - State and WebURL are value types
	entity.AddAttribute(AttrState, issue.State)
	entity.AddAttribute(AttrWebURL, issue.WebURL)

	if issue.CreatedAt != nil {
		entity.AddAttribute(AttrCreatedAt, issue.CreatedAt.Format(time.RFC3339))
	}
	if issue.UpdatedAt != nil {
		entity.AddAttribute(AttrUpdatedAt, issue.UpdatedAt.Format(time.RFC3339))
	}
	if len(issue.Labels) > 0 {
		entity.AddAttribute(AttrLabels, []string(issue.Labels))
	}

	// Add project context
	if project != nil {
		entity.AddAttribute(AttrProjectName, project.Name)
		entity.AddAttribute(AttrProjectPath, project.PathWithNamespace)
		// Add relationship to parent project
		entity.AddRelationship(RelProject, p.BuildEntityID(formatProjectID(project)).String())
	}

	// Add assignee - Use Assignees slice (Assignee is deprecated)
	if len(issue.Assignees) > 0 && issue.Assignees[0] != nil {
		entity.AddAttribute(AttrAssignee, issue.Assignees[0].Username)
		entity.AddSearchToken(issue.Assignees[0].Username)
	}

	// Add author - Username is a value type, not pointer
	if issue.Author != nil {
		entity.AddAttribute(AttrAuthor, issue.Author.Username)
	}

	// Add search tokens
	entity.AddSearchToken(issue.Title)
	if issue.Description != "" {
		entity.AddSearchToken(issue.Description)
	}
	for _, label := range issue.Labels {
		entity.AddSearchToken(label)
	}

	return entity
}

// shouldSearchIssues returns true if we should search issues for the given project.
func (p *Provider) shouldSearchIssues(projectID string) bool {
	if !p.searchIssues {
		return false
	}
	if len(p.configuredProjects) == 0 {
		return false
	}
	return p.configuredProjects[projectID]
}

// parseResourceID parses a resource ID into project ID and optional issue IID.
// Returns (projectID, issueIID, hasIssue).
func parseResourceID(resourceID string) (string, int, bool) {
	// Try to parse as "projectID/issueIID"
	var projID string
	var issueIID int
	n, err := fmt.Sscanf(resourceID, "%s/%d", &projID, &issueIID)
	if err == nil && n == 2 {
		return projID, issueIID, true
	}
	return resourceID, 0, false
}

// formatProjectID formats a project ID for storage.
func formatProjectID(project *gitlab.Project) string {
	return project.PathWithNamespace
}

// discoverWithLimit performs discovery with a limit on the number of items.
func (p *Provider) discoverWithLimit(ctx context.Context, limit int) ([]types.Entity, error) {
	opts := &gitlab.ListProjectsOptions{
		OrderBy: gitlab.Ptr("last_activity_at"),
		Sort:    gitlab.Ptr("desc"),
	}
	opts.ListOptions.PerPage = int64(limit)

	projects, _, err := p.client.Projects.ListProjects(opts)
	if err != nil {
		return nil, fmt.Errorf("list projects failed: %w", err)
	}

	entities := make([]types.Entity, 0, len(projects))
	for _, proj := range projects {
		entities = append(entities, p.projectToEntity(proj))
	}

	return entities, nil
}

// AttributeExtensions returns provider-specific attribute extensions.
func (p *Provider) AttributeExtensions(ctx context.Context) map[string]types.AttributeDef {
	extensions := map[string]types.AttributeDef{
		AttrVisibility: {
			Name: "visibility",
			Type: types.AttributeTypeString,
			UI: types.UIConfig{
				Widget: "select",
				Icon:   "Eye",
				Group:  "gitlab",
				Label:  "Visibility",
			},
			Filter: types.FilterConfig{
				SupportsEq: true,
			},
		},
		AttrArchived: {
			Name: "archived",
			Type: types.AttributeTypeBool,
			UI: types.UIConfig{
				Widget: "checkbox",
				Icon:   "Archive",
				Group:  "gitlab",
				Label:  "Archived",
			},
			Filter: types.FilterConfig{
				SupportsEq: true,
			},
		},
	}

	if p.searchIssues {
		extensions[AttrState] = types.AttributeDef{
			Name: "state",
			Type: types.AttributeTypeString,
			UI: types.UIConfig{
				Widget: "select",
				Icon:   "Circle",
				Group:  "gitlab",
				Label:  "State",
			},
			Filter: types.FilterConfig{
				SupportsEq: true,
			},
		}
		extensions[AttrLabels] = types.AttributeDef{
			Name: "labels",
			Type: types.AttributeTypeString,
			UI: types.UIConfig{
				Widget: "multiselect",
				Icon:   "Tag",
				Group:  "gitlab",
				Label:  "Labels",
			},
			Filter: types.FilterConfig{
				SupportsEq:  true,
				Cacheable:   true,
				CacheTTL:    1 * time.Hour,
			},
		}
	}

	return extensions
}

// Log returns a logger for this provider.
func (p *Provider) Log() *zerolog.Logger {
	logger := zerolog.Nop()
	return &logger
}
