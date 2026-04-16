package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func New(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

type Application struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (c *Client) CreateApplication(ctx context.Context, app *Application) (*Application, error) {
	var result Application
	err := c.do(ctx, "POST", "/application-srv/v1/applications", app, &result)
	return &result, err
}

func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	var result Application
	err := c.do(ctx, "GET", "/application-srv/v1/applications/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateApplication(ctx context.Context, id string, app *Application) (*Application, error) {
	var result Application
	err := c.do(ctx, "PUT", "/application-srv/v1/applications/"+id, app, &result)
	return &result, err
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/application-srv/v1/applications/"+id, nil, nil)
}

type Action struct {
	Name        string `json:"name"`
	Identifier  string `json:"identifier,omitempty"`
	Description string `json:"description,omitempty"`
}

type ResourceType struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description,omitempty"`
	DefaultActions   []Action `json:"default_actions"`
	IdentifierFormat *string  `json:"identifier_format,omitempty"`
	ApplicationID    string   `json:"application_id,omitempty"`
}

func (c *Client) CreateResourceType(ctx context.Context, rt *ResourceType) (*ResourceType, error) {
	var result ResourceType
	err := c.do(ctx, "POST", "/resource-srv/v1/resource-types", rt, &result)
	return &result, err
}

func (c *Client) GetResourceType(ctx context.Context, id string) (*ResourceType, error) {
	var result ResourceType
	err := c.do(ctx, "GET", "/resource-srv/v1/resource-types/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateResourceType(ctx context.Context, id string, rt *ResourceType) (*ResourceType, error) {
	var result ResourceType
	err := c.do(ctx, "PUT", "/resource-srv/v1/resource-types/"+id, rt, &result)
	return &result, err
}

func (c *Client) DeleteResourceType(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/resource-srv/v1/resource-types/"+id, nil, nil)
}

type Resource struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Type          string `json:"type"`
	ApplicationID string `json:"application_id"`
	ExternalID    string `json:"external_id,omitempty"`
}

func (c *Client) CreateResource(ctx context.Context, res *Resource) (*Resource, error) {
	var result Resource
	err := c.do(ctx, "POST", "/resource-srv/v1/resources", res, &result)
	return &result, err
}

func (c *Client) GetResource(ctx context.Context, id string) (*Resource, error) {
	var result Resource
	err := c.do(ctx, "GET", "/resource-srv/v1/resources/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateResource(ctx context.Context, id string, res *Resource) (*Resource, error) {
	var result Resource
	err := c.do(ctx, "PUT", "/resource-srv/v1/resources/"+id, res, &result)
	return &result, err
}

func (c *Client) DeleteResource(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/resource-srv/v1/resources/"+id, nil, nil)
}

type Subject struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	ApplicationIDs []string `json:"application_ids,omitempty"`
}

func (c *Client) CreateSubject(ctx context.Context, s *Subject) (*Subject, error) {
	var result Subject
	err := c.do(ctx, "POST", "/entity-srv/v1/entities", s, &result)
	return &result, err
}

func (c *Client) GetSubject(ctx context.Context, id string) (*Subject, error) {
	var result Subject
	err := c.do(ctx, "GET", "/entity-srv/v1/entities/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateSubject(ctx context.Context, id string, s *Subject) (*Subject, error) {
	var result Subject
	err := c.do(ctx, "PUT", "/entity-srv/v1/entities/"+id, s, &result)
	return &result, err
}

func (c *Client) DeleteSubject(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/entity-srv/v1/entities/"+id, nil, nil)
}

type Role struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	ApplicationIDs []string `json:"application_ids,omitempty"`
}

func (c *Client) CreateRole(ctx context.Context, r *Role) (*Role, error) {
	var result Role
	err := c.do(ctx, "POST", "/policy-srv/v1/roles", r, &result)
	return &result, err
}

func (c *Client) GetRole(ctx context.Context, id string) (*Role, error) {
	var result Role
	err := c.do(ctx, "GET", "/policy-srv/v1/roles/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateRole(ctx context.Context, id string, r *Role) (*Role, error) {
	var result Role
	err := c.do(ctx, "PUT", "/policy-srv/v1/roles/"+id, r, &result)
	return &result, err
}

func (c *Client) DeleteRole(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/policy-srv/v1/roles/"+id, nil, nil)
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (c *Client) CreateGroup(ctx context.Context, g *Group) (*Group, error) {
	var result Group
	err := c.do(ctx, "POST", "/entity-srv/v1/groups", g, &result)
	return &result, err
}

func (c *Client) GetGroup(ctx context.Context, id string) (*Group, error) {
	var result Group
	err := c.do(ctx, "GET", "/entity-srv/v1/groups/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateGroup(ctx context.Context, id string, g *Group) (*Group, error) {
	var result Group
	err := c.do(ctx, "PUT", "/entity-srv/v1/groups/"+id, g, &result)
	return &result, err
}

func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/entity-srv/v1/groups/"+id, nil, nil)
}

type PolicyResourceRef struct {
	ResourceID string   `json:"resource_id"`
	Actions    []string `json:"actions"`
}

type Policy struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Description    string              `json:"description,omitempty"`
	Effect         string              `json:"effect"`
	Resources      []PolicyResourceRef `json:"resources"`
	Priority       int                 `json:"priority,omitempty"`
	ApplicationIDs []string            `json:"application_ids,omitempty"`
}

func (c *Client) CreatePolicy(ctx context.Context, p *Policy) (*Policy, error) {
	var result Policy
	err := c.do(ctx, "POST", "/policy-srv/v1/policies", p, &result)
	return &result, err
}

func (c *Client) GetPolicy(ctx context.Context, id string) (*Policy, error) {
	var result Policy
	if err := c.do(ctx, "GET", "/policy-srv/v1/policies/"+id, nil, &result); err != nil {
		return nil, err
	}

	// The primary policy endpoint does not return attached resources (they live in
	// a separate join table). Fetch them from the dedicated endpoint and merge.
	// The response is []ResourcePolicy with fields resource_id (UUID string) and
	// actions (JSON array of strings). A non-2xx is treated as "no resources".
	type policyResourceResp struct {
		ResourceID string   `json:"resource_id"`
		Actions    []string `json:"actions"`
	}
	var attached []policyResourceResp
	if err := c.do(ctx, "GET", "/policy-srv/v1/policies/"+id+"/resources", nil, &attached); err == nil {
		refs := make([]PolicyResourceRef, len(attached))
		for i, a := range attached {
			refs[i] = PolicyResourceRef{
				ResourceID: a.ResourceID,
				Actions:    a.Actions,
			}
		}
		result.Resources = refs
	}

	return &result, nil
}

func (c *Client) UpdatePolicy(ctx context.Context, id string, p *Policy) (*Policy, error) {
	var result Policy
	err := c.do(ctx, "PUT", "/policy-srv/v1/policies/"+id, p, &result)
	return &result, err
}

func (c *Client) DeletePolicy(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/policy-srv/v1/policies/"+id, nil, nil)
}

type PolicyAssignment struct {
	PolicyIDs  []string `json:"policy_ids"`
	EntityType string   `json:"entity_type"`
	EntityID   string   `json:"entity_id"`
}

func (c *Client) AssignPolicy(ctx context.Context, a *PolicyAssignment) error {
	return c.do(ctx, "PUT", "/policy-srv/v1/policies/assign", a, nil)
}

func (c *Client) UnassignPolicy(ctx context.Context, entityType, entityID, policyID string) error {
	path := fmt.Sprintf("/policy-srv/v1/policies/unassign/%s/%s/%s", entityType, entityID, policyID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

func (c *Client) AssignRoleToSubject(ctx context.Context, subjectID, roleID string) error {
	body := map[string]string{"role_id": roleID}
	path := fmt.Sprintf("/entity-srv/v1/entities/%s/roles", subjectID)
	return c.do(ctx, "POST", path, body, nil)
}

func (c *Client) UnassignRoleFromSubject(ctx context.Context, subjectID, roleID string) error {
	path := fmt.Sprintf("/entity-srv/v1/entities/%s/roles/%s", subjectID, roleID)
	return c.do(ctx, "DELETE", path, nil, nil)
}
