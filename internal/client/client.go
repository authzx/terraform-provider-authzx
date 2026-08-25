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

// ErrNotFound is returned when the API responds 404. Resources use
// errors.Is(err, ErrNotFound) in Read to drop the resource from state.
var ErrNotFound = errors.New("not found")

// tokenRefreshLeeway is how close to expiry we refresh proactively.
const tokenRefreshLeeway = 60 * time.Second

// Client is an authenticated HTTP client for the Vengtoo API. It acquires an
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
		c.baseURL+"/v1/oauth/token",
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

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrNotFound, string(respBody))
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

type Namespace struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (c *Client) CreateNamespace(ctx context.Context, ns *Namespace) (*Namespace, error) {
	var result Namespace
	err := c.do(ctx, "POST", "/v1/namespaces", ns, &result)
	return &result, err
}

func (c *Client) GetNamespace(ctx context.Context, id string) (*Namespace, error) {
	var result Namespace
	err := c.do(ctx, "GET", "/v1/namespaces/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateNamespace(ctx context.Context, id string, ns *Namespace) (*Namespace, error) {
	var result Namespace
	err := c.do(ctx, "PUT", "/v1/namespaces/"+id, ns, &result)
	return &result, err
}

func (c *Client) DeleteNamespace(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/namespaces/"+id, nil, nil)
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
	DefaultActions   []Action `json:"actions"`
	IdentifierFormat *string  `json:"identifier_format,omitempty"`
}

func (c *Client) CreateResourceType(ctx context.Context, rt *ResourceType) (*ResourceType, error) {
	var result ResourceType
	err := c.do(ctx, "POST", "/v1/resource-types", rt, &result)
	return &result, err
}

func (c *Client) GetResourceType(ctx context.Context, id string) (*ResourceType, error) {
	var result ResourceType
	err := c.do(ctx, "GET", "/v1/resource-types/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateResourceType(ctx context.Context, id string, rt *ResourceType) (*ResourceType, error) {
	var result ResourceType
	err := c.do(ctx, "PUT", "/v1/resource-types/"+id, rt, &result)
	return &result, err
}

func (c *Client) DeleteResourceType(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/resource-types/"+id, nil, nil)
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
	err := c.do(ctx, "POST", "/v1/resources", res, &result)
	return &result, err
}

func (c *Client) GetResource(ctx context.Context, id string) (*Resource, error) {
	var result Resource
	err := c.do(ctx, "GET", "/v1/resources/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateResource(ctx context.Context, id string, res *Resource) (*Resource, error) {
	var result Resource
	err := c.do(ctx, "PUT", "/v1/resources/"+id, res, &result)
	return &result, err
}

func (c *Client) DeleteResource(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/resources/"+id, nil, nil)
}

type Subject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	// ExternalID is the customer's own identifier for the subject (e.g., a
	// user ID from their auth system). Scoped uniquely by (tenant, external_id)
	// and usable in /v1/authorize as an alternative to the UUID.
	ExternalID string `json:"external_id,omitempty"`
}

func (c *Client) CreateSubject(ctx context.Context, s *Subject) (*Subject, error) {
	var result Subject
	err := c.do(ctx, "POST", "/v1/entities", s, &result)
	return &result, err
}

func (c *Client) GetSubject(ctx context.Context, id string) (*Subject, error) {
	var result Subject
	err := c.do(ctx, "GET", "/v1/entities/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateSubject(ctx context.Context, id string, s *Subject) (*Subject, error) {
	var result Subject
	err := c.do(ctx, "PUT", "/v1/entities/"+id, s, &result)
	return &result, err
}

func (c *Client) DeleteSubject(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/entities/"+id, nil, nil)
}

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (c *Client) CreateRole(ctx context.Context, r *Role) (*Role, error) {
	var result Role
	err := c.do(ctx, "POST", "/v1/roles", r, &result)
	return &result, err
}

func (c *Client) GetRole(ctx context.Context, id string) (*Role, error) {
	var result Role
	err := c.do(ctx, "GET", "/v1/roles/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateRole(ctx context.Context, id string, r *Role) (*Role, error) {
	var result Role
	err := c.do(ctx, "PUT", "/v1/roles/"+id, r, &result)
	return &result, err
}

func (c *Client) DeleteRole(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/roles/"+id, nil, nil)
}

type PolicyResourceRef struct {
	ResourceID string   `json:"resource_id"`
	Actions    []string `json:"actions"`
}

type PolicyResourceTypeRef struct {
	ResourceTypeID string   `json:"resource_type_id"`
	Actions        []string `json:"actions"`
}

// AttrCheck is one {key, op, value} attribute comparison. Value is polymorphic
// (scalar or list), surfaced in Terraform as a JSON-encoded string.
type AttrCheck struct {
	Key   string          `json:"key"`
	Op    string          `json:"op"`
	Value json.RawMessage `json:"value"`
}

// PolicyConditions is the typed conditions object the policy API expects. Only
// the attribute-check arrays are managed via Terraform; advanced guards (time
// windows, MFA, geo, rate limits, expression trees) are set via the API.
type PolicyConditions struct {
	SubjectAttrs  []AttrCheck `json:"subject_attrs,omitempty"`
	ResourceAttrs []AttrCheck `json:"resource_attrs,omitempty"`
	ContextAttrs  []AttrCheck `json:"context_attrs,omitempty"`
}

type Policy struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description,omitempty"`
	Effect        string                  `json:"effect"`
	Resources     []PolicyResourceRef     `json:"resources"`
	ResourceTypes []PolicyResourceTypeRef `json:"resource_types,omitempty"`
	Priority      int                     `json:"priority,omitempty"`
	Actions       []string                `json:"actions,omitempty"`
	Conditions    *PolicyConditions       `json:"conditions,omitempty"`
}

func (c *Client) CreatePolicy(ctx context.Context, p *Policy) (*Policy, error) {
	var result Policy
	err := c.do(ctx, "POST", "/v1/policies", p, &result)
	return &result, err
}

func (c *Client) GetPolicy(ctx context.Context, id string) (*Policy, error) {
	var result Policy
	if err := c.do(ctx, "GET", "/v1/policies/"+id, nil, &result); err != nil {
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
	if err := c.do(ctx, "GET", "/v1/policies/"+id+"/resources", nil, &attached); err == nil {
		refs := make([]PolicyResourceRef, len(attached))
		for i, a := range attached {
			refs[i] = PolicyResourceRef{
				ResourceID: a.ResourceID,
				Actions:    a.Actions,
			}
		}
		result.Resources = refs
	}

	// Resource-type targets live in a separate join table.
	type policyResourceTypeResp struct {
		ResourceTypeID string   `json:"resource_type_id"`
		Actions        []string `json:"actions"`
	}
	var rtAttached []policyResourceTypeResp
	if err := c.do(ctx, "GET", "/v1/policies/"+id+"/resource-types", nil, &rtAttached); err == nil {
		refs := make([]PolicyResourceTypeRef, len(rtAttached))
		for i, rt := range rtAttached {
			refs[i] = PolicyResourceTypeRef{
				ResourceTypeID: rt.ResourceTypeID,
				Actions:        rt.Actions,
			}
		}
		result.ResourceTypes = refs
	}

	return &result, nil
}

func (c *Client) UpdatePolicy(ctx context.Context, id string, p *Policy) (*Policy, error) {
	var result Policy
	err := c.do(ctx, "PUT", "/v1/policies/"+id, p, &result)
	return &result, err
}

func (c *Client) DeletePolicy(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/policies/"+id, nil, nil)
}

type PolicyAssignment struct {
	PolicyIDs  []string `json:"policy_ids"`
	EntityType string   `json:"entity_type"`
	EntityID   string   `json:"entity_id"`
	StartsAt   *string  `json:"starts_at,omitempty"`
	ExpiresAt  *string  `json:"expires_at,omitempty"`
}

func (c *Client) AssignPolicy(ctx context.Context, a *PolicyAssignment) error {
	return c.do(ctx, "POST", "/v1/policies/assign", a, nil)
}

func (c *Client) UnassignPolicy(ctx context.Context, entityType, entityID, policyID string) error {
	body := &PolicyAssignment{
		PolicyIDs:  []string{policyID},
		EntityType: entityType,
		EntityID:   entityID,
	}
	return c.do(ctx, "POST", "/v1/policies/unassign", body, nil)
}

func (c *Client) AssignRoleToSubject(ctx context.Context, subjectID, roleID string) error {
	body := map[string]string{"role_id": roleID}
	path := fmt.Sprintf("/v1/entities/%s/roles", subjectID)
	return c.do(ctx, "POST", path, body, nil)
}

func (c *Client) UnassignRoleFromSubject(ctx context.Context, subjectID, roleID string) error {
	path := fmt.Sprintf("/v1/entities/%s/roles/%s", subjectID, roleID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

type Group struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

func (c *Client) CreateGroup(ctx context.Context, g *Group) (*Group, error) {
	var result Group
	err := c.do(ctx, "POST", "/v1/groups", g, &result)
	return &result, err
}

func (c *Client) GetGroup(ctx context.Context, id string) (*Group, error) {
	var result Group
	err := c.do(ctx, "GET", "/v1/groups/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateGroup(ctx context.Context, id string, g *Group) (*Group, error) {
	var result Group
	err := c.do(ctx, "PUT", "/v1/groups/"+id, g, &result)
	return &result, err
}

func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/groups/"+id, nil, nil)
}

// SubjectAttributeDefinition omits no optional field: the update endpoint takes
// pointers and treats an absent key as "unchanged", so values must be sent
// explicitly for a cleared attribute to actually clear.
type SubjectAttributeDefinition struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	EnumOptions  []string `json:"enum_options"`
	Required     bool     `json:"required"`
	SubjectTypes []string `json:"subject_types"`
}

func (c *Client) CreateSubjectAttribute(ctx context.Context, a *SubjectAttributeDefinition) (*SubjectAttributeDefinition, error) {
	var result SubjectAttributeDefinition
	err := c.do(ctx, "POST", "/v1/subject-attributes", a, &result)
	return &result, err
}

func (c *Client) GetSubjectAttribute(ctx context.Context, id string) (*SubjectAttributeDefinition, error) {
	var result SubjectAttributeDefinition
	err := c.do(ctx, "GET", "/v1/subject-attributes/"+id, nil, &result)
	return &result, err
}

func (c *Client) UpdateSubjectAttribute(ctx context.Context, id string, a *SubjectAttributeDefinition) (*SubjectAttributeDefinition, error) {
	var result SubjectAttributeDefinition
	err := c.do(ctx, "PUT", "/v1/subject-attributes/"+id, a, &result)
	return &result, err
}

func (c *Client) DeleteSubjectAttribute(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/v1/subject-attributes/"+id, nil, nil)
}

func (c *Client) AddEntityToGroup(ctx context.Context, groupID, entityID string) error {
	body := map[string][]string{"entity_ids": {entityID}}
	path := fmt.Sprintf("/v1/groups/%s/entities", groupID)
	return c.do(ctx, "POST", path, body, nil)
}

func (c *Client) RemoveEntityFromGroup(ctx context.Context, groupID, entityID string) error {
	path := fmt.Sprintf("/v1/groups/%s/entities/%s", groupID, entityID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

// ListGroupEntities returns a bare JSON array, not the paginated envelope the
// top-level list endpoints use.
func (c *Client) ListGroupEntities(ctx context.Context, groupID string) ([]Subject, error) {
	var result []Subject
	path := fmt.Sprintf("/v1/groups/%s/entities", groupID)
	err := c.do(ctx, "GET", path, nil, &result)
	return result, err
}

func (c *Client) AssignRoleToGroup(ctx context.Context, groupID, roleID string) error {
	body := map[string]string{"role_id": roleID}
	path := fmt.Sprintf("/v1/groups/%s/roles", groupID)
	return c.do(ctx, "POST", path, body, nil)
}

func (c *Client) RemoveRoleFromGroup(ctx context.Context, groupID, roleID string) error {
	path := fmt.Sprintf("/v1/groups/%s/roles/%s", groupID, roleID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

// ListGroupRoles returns a bare JSON array.
func (c *Client) ListGroupRoles(ctx context.Context, groupID string) ([]Role, error) {
	var result []Role
	path := fmt.Sprintf("/v1/groups/%s/roles", groupID)
	err := c.do(ctx, "GET", path, nil, &result)
	return result, err
}

type GroupParentEdge struct {
	ID            string `json:"id"`
	ChildGroupID  string `json:"child_group_id"`
	ParentGroupID string `json:"parent_group_id"`
}

// AddGroupParent returns 201 with an empty body, so there is no edge ID to read back.
func (c *Client) AddGroupParent(ctx context.Context, groupID, parentID string) error {
	body := map[string]string{"parent_id": parentID}
	path := fmt.Sprintf("/v1/groups/%s/parents", groupID)
	return c.do(ctx, "POST", path, body, nil)
}

func (c *Client) RemoveGroupParent(ctx context.Context, groupID, parentID string) error {
	path := fmt.Sprintf("/v1/groups/%s/parents/%s", groupID, parentID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

func (c *Client) ListGroupParents(ctx context.Context, groupID string) ([]GroupParentEdge, error) {
	var result struct {
		Data []GroupParentEdge `json:"data"`
	}
	path := fmt.Sprintf("/v1/groups/%s/parents", groupID)
	err := c.do(ctx, "GET", path, nil, &result)
	return result.Data, err
}

type RoleParentEdge struct {
	ID           string `json:"id"`
	ChildRoleID  string `json:"child_role_id"`
	ParentRoleID string `json:"parent_role_id"`
}

// AddRoleParent returns 201 with an empty body, so there is no edge ID to read back.
func (c *Client) AddRoleParent(ctx context.Context, roleID, parentID string) error {
	body := map[string]string{"parent_id": parentID}
	path := fmt.Sprintf("/v1/roles/%s/parents", roleID)
	return c.do(ctx, "POST", path, body, nil)
}

func (c *Client) RemoveRoleParent(ctx context.Context, roleID, parentID string) error {
	path := fmt.Sprintf("/v1/roles/%s/parents/%s", roleID, parentID)
	return c.do(ctx, "DELETE", path, nil, nil)
}

func (c *Client) ListRoleParents(ctx context.Context, roleID string) ([]RoleParentEdge, error) {
	var result struct {
		Data []RoleParentEdge `json:"data"`
	}
	path := fmt.Sprintf("/v1/roles/%s/parents", roleID)
	err := c.do(ctx, "GET", path, nil, &result)
	return result.Data, err
}

// listQuery builds the shared list query string. search is a substring match
// server-side, so callers must still exact-match the result.
func listQuery(search string) string {
	return "?search=" + url.QueryEscape(search) + "&limit=100"
}

func (c *Client) ListNamespaces(ctx context.Context, search string) ([]Namespace, error) {
	var result struct {
		Data []Namespace `json:"data"`
	}
	err := c.do(ctx, "GET", "/v1/namespaces"+listQuery(search), nil, &result)
	return result.Data, err
}

func (c *Client) ListRoles(ctx context.Context, search string) ([]Role, error) {
	var result struct {
		Data []Role `json:"data"`
	}
	err := c.do(ctx, "GET", "/v1/roles"+listQuery(search), nil, &result)
	return result.Data, err
}

// ListResourceTypes ignores search: /v1/resource-types returns a bare array and
// supports no query parameters. Callers filter client-side.
func (c *Client) ListResourceTypes(ctx context.Context, _ string) ([]ResourceType, error) {
	var result []ResourceType
	err := c.do(ctx, "GET", "/v1/resource-types", nil, &result)
	return result, err
}

func (c *Client) ListSubjects(ctx context.Context, search string) ([]Subject, error) {
	var result struct {
		Data []Subject `json:"data"`
	}
	err := c.do(ctx, "GET", "/v1/entities"+listQuery(search), nil, &result)
	return result.Data, err
}

func (c *Client) ListPolicies(ctx context.Context, search string) ([]Policy, error) {
	var result struct {
		Data []Policy `json:"data"`
	}
	err := c.do(ctx, "GET", "/v1/policies"+listQuery(search), nil, &result)
	return result.Data, err
}

func (c *Client) ListGroups(ctx context.Context, search string) ([]Group, error) {
	var result struct {
		Data []Group `json:"data"`
	}
	err := c.do(ctx, "GET", "/v1/groups"+listQuery(search), nil, &result)
	return result.Data, err
}

// ListSubjectAttributes ignores search: /v1/subject-attributes supports no query
// parameters and returns an {attributes: [...]} envelope. Callers filter client-side.
func (c *Client) ListSubjectAttributes(ctx context.Context, _ string) ([]SubjectAttributeDefinition, error) {
	var result struct {
		Attributes []SubjectAttributeDefinition `json:"attributes"`
	}
	err := c.do(ctx, "GET", "/v1/subject-attributes", nil, &result)
	return result.Attributes, err
}
