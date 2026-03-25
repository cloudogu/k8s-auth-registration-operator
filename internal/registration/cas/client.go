package cas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

const registeredServicesPath = "/actuator/registeredServices"

type ClientConfig struct {
	BaseURL  string
	Username string
	Password string
	Timeout  time.Duration
}

type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

func NewClient(cfg ClientConfig, httpClient *http.Client) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base URL must not be empty")
	}

	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", cfg.BaseURL, err)
	}

	if strings.TrimSpace(cfg.Username) == "" {
		return nil, fmt.Errorf("username must not be empty")
	}
	if strings.TrimSpace(cfg.Password) == "" {
		return nil, fmt.Errorf("password must not be empty")
	}

	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		baseURL:    baseURL,
		username:   cfg.Username,
		password:   cfg.Password,
		httpClient: httpClient,
	}, nil
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) ListServices(ctx context.Context) ([]RegisteredService, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+registeredServicesPath, nil)
	if err != nil {
		return nil, err
	}

	body, err := c.do(req, http.StatusOK)
	if err != nil {
		return nil, err
	}

	if len(body) == 0 {
		return nil, nil
	}

	var services registeredServicesList
	if err := json.Unmarshal(body, &services); err != nil {
		return nil, fmt.Errorf("failed to decode list services response: %w", err)
	}

	return []RegisteredService(services), nil
}

func (c *Client) CreateService(ctx context.Context, service RegisteredService) (RegisteredService, error) {
	return c.writeService(ctx, http.MethodPost, service)
}

func (c *Client) UpdateService(ctx context.Context, service RegisteredService) (RegisteredService, error) {
	return c.writeService(ctx, http.MethodPut, service)
}

func (c *Client) DeleteService(ctx context.Context, id int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s%s/%d", c.baseURL, registeredServicesPath, id), nil)
	if err != nil {
		return err
	}

	_, err = c.do(req, http.StatusOK, http.StatusNoContent, http.StatusNotFound)
	if err != nil {
		var apiErr *apiError
		if ok := errorAs(err, &apiErr); ok && apiErr.StatusCode == http.StatusNotFound {
			return nil
		}
		return err
	}

	return nil
}

func (c *Client) writeService(ctx context.Context, method string, service RegisteredService) (RegisteredService, error) {
	payload, err := json.Marshal(service)
	if err != nil {
		return RegisteredService{}, fmt.Errorf("failed to encode %s payload: %w", strings.ToLower(method), err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+registeredServicesPath, bytes.NewReader(payload))
	if err != nil {
		return RegisteredService{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	body, err := c.do(req, http.StatusOK, http.StatusCreated)
	if err != nil {
		return RegisteredService{}, err
	}

	if len(body) == 0 {
		return service, nil
	}

	var decoded RegisteredService
	if err := json.Unmarshal(body, &decoded); err != nil {
		return RegisteredService{}, fmt.Errorf("failed to decode %s response: %w", strings.ToLower(method), err)
	}

	return decoded, nil
}

func (c *Client) do(req *http.Request, expectedStatusCodes ...int) ([]byte, error) {
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s failed: %w", req.Method, req.URL.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s %s response: %w", req.Method, req.URL.String(), err)
	}

	if slices.Contains(expectedStatusCodes, resp.StatusCode) {
		return body, nil
	}

	return nil, &apiError{
		Method:     req.Method,
		URL:        req.URL.String(),
		StatusCode: resp.StatusCode,
		Body:       strings.TrimSpace(string(body)),
	}
}

type apiError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("%s %s returned unexpected status %d", e.Method, e.URL, e.StatusCode)
	}

	return fmt.Sprintf("%s %s returned unexpected status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
}

func errorAs(err error, target **apiError) bool {
	apiErr, ok := err.(*apiError)
	if !ok {
		return false
	}

	*target = apiErr
	return true
}
