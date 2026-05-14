// Package keycloak provides backend-only helpers for Keycloak Admin API calls.
package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultTimeout         = 30 * time.Second
	defaultPageSize        = 100
	tokenRefreshSkew       = 30 * time.Second
	maxUnauthorizedRetries = 1
)

// ErrRoleNotFound is returned when Keycloak reports the requested realm role
// does not exist.
var ErrRoleNotFound = errors.New("keycloak: realm role not found")

// Config holds the Keycloak Admin API and client-credentials settings.
type Config struct {
	BaseURL      string
	Realm        string
	TokenURL     string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	Timeout      time.Duration
}

// Client calls the Keycloak Admin API with a cached service-account token.
type Client struct {
	cfg        Config
	httpClient *http.Client
	now        func() time.Time

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

// UsersByRealmRoleOptions controls resolver pagination.
type UsersByRealmRoleOptions struct {
	PageSize int
}

// User is the normalized backend representation returned by role lookups.
type User struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Enabled   bool   `json:"enabled"`
}

// UpstreamError wraps non-success responses from Keycloak.
type UpstreamError struct {
	Operation  string
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *UpstreamError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		return fmt.Sprintf("keycloak: %s returned HTTP %d", e.Operation, e.StatusCode)
	}
	return fmt.Sprintf("keycloak: %s returned HTTP %d: %s", e.Operation, e.StatusCode, body)
}

// New creates a Keycloak Admin API client.
func New(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Realm = strings.TrimSpace(cfg.Realm)
	cfg.TokenURL = strings.TrimSpace(cfg.TokenURL)

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		now:        time.Now,
	}
}

// UsersByRealmRole resolves enabled users with email addresses who have a realm
// role directly or through Keycloak group membership. Composite roles are not
// expanded.
func (c *Client) UsersByRealmRole(ctx context.Context, roleName string, opts UsersByRealmRoleOptions) ([]User, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return nil, fmt.Errorf("keycloak: role name is required")
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	seenUsers := make(map[string]User)
	directUsers, err := c.roleUsers(ctx, roleName, pageSize)
	if err != nil {
		return nil, err
	}
	for _, raw := range directUsers {
		addUser(seenUsers, raw)
	}

	groups, err := c.roleGroups(ctx, roleName, pageSize)
	if err != nil {
		return nil, err
	}
	seenGroups := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		if err := c.collectGroupUsers(ctx, group.ID, pageSize, seenGroups, seenUsers); err != nil {
			return nil, err
		}
	}

	users := make([]User, 0, len(seenUsers))
	for _, user := range seenUsers {
		users = append(users, user)
	}
	sort.SliceStable(users, func(i, j int) bool {
		leftEmail := strings.ToLower(users[i].Email)
		rightEmail := strings.ToLower(users[j].Email)
		if leftEmail != rightEmail {
			return leftEmail < rightEmail
		}
		leftName := strings.ToLower(users[i].Name)
		rightName := strings.ToLower(users[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		return users[i].ID < users[j].ID
	})

	return users, nil
}

func (c *Client) validate() error {
	missing := make([]string, 0)
	if c.cfg.BaseURL == "" {
		missing = append(missing, "base_url")
	}
	if c.cfg.Realm == "" {
		missing = append(missing, "realm")
	}
	if c.cfg.TokenURL == "" {
		missing = append(missing, "token_url")
	}
	if c.cfg.ClientID == "" {
		missing = append(missing, "client_id")
	}
	if c.cfg.ClientSecret == "" {
		missing = append(missing, "client_secret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("keycloak: missing config: %s", strings.Join(missing, ", "))
	}
	return nil
}

func (c *Client) roleUsers(ctx context.Context, roleName string, pageSize int) ([]keycloakUser, error) {
	path := c.adminPath("roles", roleName, "users")
	query := url.Values{"briefRepresentation": {"true"}}
	var users []keycloakUser
	err := c.collectPages(ctx, path, query, pageSize, &users)
	if err != nil {
		return nil, mapRoleLookupError(err, roleName)
	}
	return users, nil
}

func (c *Client) roleGroups(ctx context.Context, roleName string, pageSize int) ([]keycloakGroup, error) {
	path := c.adminPath("roles", roleName, "groups")
	query := url.Values{"briefRepresentation": {"true"}}
	var groups []keycloakGroup
	err := c.collectPages(ctx, path, query, pageSize, &groups)
	if err != nil {
		return nil, mapRoleLookupError(err, roleName)
	}
	return groups, nil
}

func (c *Client) collectGroupUsers(ctx context.Context, groupID string, pageSize int, seenGroups map[string]struct{}, users map[string]User) error {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	if _, ok := seenGroups[groupID]; ok {
		return nil
	}
	seenGroups[groupID] = struct{}{}

	memberPath := c.adminPath("groups", groupID, "members")
	memberQuery := url.Values{"briefRepresentation": {"true"}}
	var members []keycloakUser
	if err := c.collectPages(ctx, memberPath, memberQuery, pageSize, &members); err != nil {
		return err
	}
	for _, raw := range members {
		addUser(users, raw)
	}

	childPath := c.adminPath("groups", groupID, "children")
	childQuery := url.Values{"briefRepresentation": {"true"}}
	var children []keycloakGroup
	if err := c.collectPages(ctx, childPath, childQuery, pageSize, &children); err != nil {
		return err
	}
	for _, child := range children {
		if err := c.collectGroupUsers(ctx, child.ID, pageSize, seenGroups, users); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) collectPages(ctx context.Context, path string, baseQuery url.Values, pageSize int, dest any) error {
	first := 0
	for {
		query := cloneValues(baseQuery)
		query.Set("first", fmt.Sprintf("%d", first))
		query.Set("max", fmt.Sprintf("%d", pageSize))

		pageLen, err := c.getJSONPage(ctx, path, query, dest)
		if err != nil {
			return err
		}
		if pageLen < pageSize {
			return nil
		}
		first += pageSize
	}
}

func (c *Client) getJSONPage(ctx context.Context, path string, query url.Values, dest any) (int, error) {
	switch out := dest.(type) {
	case *[]keycloakUser:
		var page []keycloakUser
		if err := c.getJSON(ctx, path, query, &page); err != nil {
			return 0, err
		}
		*out = append(*out, page...)
		return len(page), nil
	case *[]keycloakGroup:
		var page []keycloakGroup
		if err := c.getJSON(ctx, path, query, &page); err != nil {
			return 0, err
		}
		*out = append(*out, page...)
		return len(page), nil
	default:
		return 0, fmt.Errorf("keycloak: unsupported pagination destination %T", dest)
	}
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, dest any) error {
	resp, err := c.do(ctx, http.MethodGet, path, query)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		return &UpstreamError{
			Operation:  "admin api",
			Method:     http.MethodGet,
			Path:       path,
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("keycloak: decode admin response %s: %w", path, err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	fullURL := c.cfg.BaseURL + path
	if encoded := query.Encode(); encoded != "" {
		fullURL += "?" + encoded
	}

	for attempt := 0; attempt <= maxUnauthorizedRetries; attempt++ {
		token, err := c.token(ctx)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("keycloak: create admin request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusUnauthorized || attempt == maxUnauthorizedRetries {
			return resp, nil
		}
		resp.Body.Close()
		c.invalidateToken()
	}

	return nil, fmt.Errorf("keycloak: exhausted unauthorized retries")
}

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if c.accessToken != "" && now.Before(c.expiry.Add(-tokenRefreshSkew)) {
		return c.accessToken, nil
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("keycloak: create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("keycloak: token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", &UpstreamError{
			Operation:  "token endpoint",
			Method:     http.MethodPost,
			Path:       c.cfg.TokenURL,
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("keycloak: decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("keycloak: token response missing access_token")
	}

	c.accessToken = tokenResp.AccessToken
	c.expiry = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

func (c *Client) invalidateToken() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.accessToken = ""
	c.expiry = time.Time{}
}

func (c *Client) adminPath(parts ...string) string {
	segments := []string{"admin", "realms", c.cfg.Realm}
	segments = append(segments, parts...)
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, url.PathEscape(segment))
	}
	return "/" + strings.Join(escaped, "/")
}

func addUser(users map[string]User, raw keycloakUser) {
	user, ok := normalizeUser(raw)
	if !ok {
		return
	}
	if _, exists := users[user.ID]; exists {
		return
	}
	users[user.ID] = user
}

func normalizeUser(raw keycloakUser) (User, bool) {
	id := strings.TrimSpace(raw.ID)
	email := strings.ToLower(strings.TrimSpace(raw.Email))
	if id == "" || email == "" {
		return User{}, false
	}
	enabled := true
	if raw.Enabled != nil {
		enabled = *raw.Enabled
	}
	if !enabled {
		return User{}, false
	}

	firstName := strings.TrimSpace(raw.FirstName)
	lastName := strings.TrimSpace(raw.LastName)
	username := strings.TrimSpace(raw.Username)
	name := strings.TrimSpace(strings.Join([]string{firstName, lastName}, " "))
	if name == "" {
		name = username
	}
	if name == "" {
		name = email
	}

	return User{
		ID:        id,
		Username:  username,
		FirstName: firstName,
		LastName:  lastName,
		Name:      name,
		Email:     email,
		Enabled:   enabled,
	}, true
}

func mapRoleLookupError(err error, roleName string) error {
	var upstreamErr *UpstreamError
	if errors.As(err, &upstreamErr) && upstreamErr.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%w: %s", ErrRoleNotFound, roleName)
	}
	return err
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, entries := range values {
		cloned[key] = append([]string(nil), entries...)
	}
	return cloned
}

type keycloakUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Enabled   *bool  `json:"enabled"`
}

type keycloakGroup struct {
	ID string `json:"id"`
}
