package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrAuthentication is returned when the token endpoint rejects the client
// credentials. The provider surfaces this as a diagnostic rather than a raw
// transport error.
var ErrAuthentication = errors.New("authentication failed: check client_id/client_secret")

// tokenRefreshLeeway is how close to expiry we refresh proactively.
const tokenRefreshLeeway = 60 * time.Second

// Client is an authenticated HTTP client for the AuthzX API. It acquires an
// access token via the OAuth2 Client Credentials flow (RFC 6749 §4.4) and
// transparently refreshes it on expiry or on a 401 response.
type Client struct {
	clientID     string
	clientSecret string
	baseURL      string
	httpClient   *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// New constructs a Client. The token is not fetched here — it is obtained
// lazily on the first API call (or via Authenticate() to surface credential
// errors at provider configure time).
func New(clientID, clientSecret, baseURL string) *Client {
	return &Client{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      strings.TrimRight(baseURL, "/"),
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

// tokenResponse models a successful OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// tokenErrorResponse models an OAuth2 token endpoint error response (RFC 6749 §5.2).
type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// Authenticate forces a token exchange. Call this at provider Configure() time
// to fail fast on bad credentials.
func (c *Client) Authenticate(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshTokenLocked(ctx)
}

// token returns a valid access token, refreshing if within the leeway of expiry.
func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Until(c.expiresAt) > tokenRefreshLeeway {
		return c.accessToken, nil
	}
	if err := c.refreshTokenLocked(ctx); err != nil {
		return "", err
	}
	return c.accessToken, nil
}

// refreshTokenLocked performs the client_credentials grant. Caller must hold c.mu.
func (c *Client) refreshTokenLocked(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+"/identity-srv/v1/oauth/token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrAuthentication
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var tErr tokenErrorResponse
		if jsonErr := json.Unmarshal(body, &tErr); jsonErr == nil && tErr.Error != "" {
			if tErr.Error == "invalid_client" {
				return ErrAuthentication
			}
			return fmt.Errorf("token endpoint error (status %d): %s %s", resp.StatusCode, tErr.Error, tErr.ErrorDescription)
		}
		return fmt.Errorf("token endpoint error (status %d): %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return fmt.Errorf("token endpoint returned empty access_token")
	}

	c.accessToken = tok.AccessToken
	// Fall back to 1h if the server omits expires_in.
	if tok.ExpiresIn > 0 {
		c.expiresAt = time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	} else {
		c.expiresAt = time.Now().Add(time.Hour)
	}
	return nil
}

// invalidateToken clears the cached token so the next call forces a refresh.
// Used when the API returns 401 on a non-token request (token likely revoked).
func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accessToken = ""
	c.expiresAt = time.Time{}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	return c.doWithRetry(ctx, method, path, body, result, true)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body interface{}, result interface{}, retryOn401 bool) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	tok, err := c.token(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// If the server rejected our token, invalidate and retry once with a fresh one.
	if resp.StatusCode == http.StatusUnauthorized && retryOn401 {
		c.invalidateToken()
		return c.doWithRetry(ctx, method, path, body, result, false)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
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
	Actions        []string            `json:"actions,omitempty"`
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

	// The primary endpoint also does not return application_ids (the field is
	// json:"-" on the backend model). Fetch from the dedicated endpoint.
	type policyAppResp struct {
		ApplicationID string `json:"application_id"`
	}
	var apps []policyAppResp
	if err := c.do(ctx, "GET", "/policy-srv/v1/policies/"+id+"/applications", nil, &apps); err == nil {
		ids := make([]string, len(apps))
		for i, a := range apps {
			ids[i] = a.ApplicationID
		}
		result.ApplicationIDs = ids
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
