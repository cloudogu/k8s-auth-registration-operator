package cas

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientListServicesSupportsCASCollectionPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/cas/actuator/registeredServices", r.URL.Path)
		username, password, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "cas-user", username)
		assert.Equal(t, "cas-password", password)
		w.Header().Set("Content-Type", contentTypeJSON)
		_, _ = w.Write([]byte(`["java.util.ArrayList",[{"id":42,"name":"app","serviceId":"^https://app.example.com"}]]`))
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		BaseURL:  server.URL + "/cas",
		Username: "cas-user",
		Password: "cas-password",
		Timeout:  time.Second,
	}, server.Client())
	require.NoError(t, err)

	services, err := client.ListServices(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	assert.Equal(t, "app", services[0].Name)
	assert.Equal(t, int64(42), services[0].ID)
}

func TestNewClientRejectsInvalidConfig(t *testing.T) {
	t.Run("returns error for empty base URL", func(t *testing.T) {
		_, err := NewClient(ClientConfig{}, nil)

		require.Error(t, err)
		assert.ErrorContains(t, err, "base URL must not be empty")
	})

	t.Run("uses default timeout and trims trailing slash from base URL", func(t *testing.T) {
		previousTimeout := defaultClientTimeout
		t.Cleanup(func() {
			defaultClientTimeout = previousTimeout
		})

		defaultClientTimeout = 7 * time.Second

		client, err := NewClient(ClientConfig{
			BaseURL:  "https://cas.example.com/cas/",
			Username: "cas-user",
			Password: "cas-password",
		}, nil)

		require.NoError(t, err)
		require.NotNil(t, client)
		assert.Equal(t, "https://cas.example.com/cas", client.BaseURL())
		require.NotNil(t, client.httpClient)
		assert.Equal(t, 7*time.Second, client.httpClient.Timeout)
	})

	t.Run("rejects invalid base URL", func(t *testing.T) {
		client, err := NewClient(ClientConfig{
			BaseURL:  "http://[::1",
			Username: "cas-user",
			Password: "cas-password",
		}, nil)

		require.Error(t, err)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "invalid base URL")
	})

	t.Run("rejects blank username", func(t *testing.T) {
		client, err := NewClient(ClientConfig{
			BaseURL:  "https://cas.example.com/cas",
			Username: "   ",
			Password: "cas-password",
		}, nil)

		require.Error(t, err)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "username must not be empty")
	})

	t.Run("rejects blank password", func(t *testing.T) {
		client, err := NewClient(ClientConfig{
			BaseURL:  "https://cas.example.com/cas",
			Username: "cas-user",
			Password: "   ",
		}, nil)

		require.Error(t, err)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "password must not be empty")
	})
}

func TestClientDeleteTreatsNotFoundAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("cas-user:cas-password")), r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		BaseURL:  server.URL,
		Username: "cas-user",
		Password: "cas-password",
		Timeout:  time.Second,
	}, server.Client())
	require.NoError(t, err)

	err = client.DeleteService(context.Background(), 77)

	require.NoError(t, err)
}

func TestClientDeleteServiceSendsJSONContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/actuator/registeredServices/77", r.URL.Path)
		// CAS >= 7.3 rejects delete requests without a matching Content-Type with 415.
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newClientForHTTPServerTest(t, server)

	err := client.DeleteService(context.Background(), 77)

	require.NoError(t, err)
}

func TestClientListServicesSupportsObjectPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", contentTypeJSON)
		_, _ = w.Write([]byte(`{"services":[{"id":21,"name":"app","serviceId":"^https://app.example.com"}]}`))
	}))
	defer server.Close()

	client := newClientForHTTPServerTest(t, server)

	services, err := client.ListServices(context.Background())

	require.NoError(t, err)
	require.Len(t, services, 1)
	assert.Equal(t, int64(21), services[0].ID)
	assert.Equal(t, "app", services[0].Name)
}

func TestClientListServicesReturnsDecodeErrorForInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"services":`))
	}))
	defer server.Close()

	client := newClientForHTTPServerTest(t, server)

	services, err := client.ListServices(context.Background())

	require.Error(t, err)
	assert.Nil(t, services)
	assert.ErrorContains(t, err, "failed to decode list services response")
}

func TestClientListServicesReturnsDecodeErrorForEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newClientForHTTPServerTest(t, server)

	services, err := client.ListServices(context.Background())

	require.Error(t, err)
	assert.Nil(t, services)
	assert.ErrorContains(t, err, "failed to decode list services response")
}

func TestClientCreateService(t *testing.T) {
	t.Run("sends post request and decodes response body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.Header().Set("Content-Type", contentTypeJSON)
			_, _ = w.Write([]byte(`{"id":11,"name":"created","serviceId":"^https://created.example.com"}`))
		}))
		defer server.Close()

		client := newClientForHTTPServerTest(t, server)

		service, err := client.CreateService(context.Background(), RegisteredService{Name: "created"})

		require.NoError(t, err)
		assert.Equal(t, int64(11), service.ID)
		assert.Equal(t, "created", service.Name)
	})

	t.Run("returns original payload when server responds without body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		client := newClientForHTTPServerTest(t, server)
		payload := RegisteredService{Name: "created"}

		service, err := client.CreateService(context.Background(), payload)

		require.NoError(t, err)
		assert.Equal(t, payload, service)
	})
}

func TestClientUpdateService(t *testing.T) {
	t.Run("sends put request and returns decode error for invalid response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			_, _ = w.Write([]byte(`{"id":`))
		}))
		defer server.Close()

		client := newClientForHTTPServerTest(t, server)

		service, err := client.UpdateService(context.Background(), RegisteredService{Name: "updated"})

		require.Error(t, err)
		assert.Empty(t, service)
		assert.ErrorContains(t, err, "failed to decode put response")
	})
}

func TestClientDeleteServiceReturnsAPIErrorForUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer server.Close()

	client := newClientForHTTPServerTest(t, server)

	err := client.DeleteService(context.Background(), 77)

	require.Error(t, err)
	assert.ErrorContains(t, err, "returned unexpected status 500")
}

func TestAPIErrorFormattingAndErrorAs(t *testing.T) {
	t.Run("formats message with and without body", func(t *testing.T) {
		errWithBody := &apiError{Method: http.MethodGet, URL: "https://cas.example.com", StatusCode: 500, Body: "boom"}
		errWithoutBody := &apiError{Method: http.MethodGet, URL: "https://cas.example.com", StatusCode: 500}

		assert.Equal(t, "GET https://cas.example.com returned unexpected status 500: boom", errWithBody.Error())
		assert.Equal(t, "GET https://cas.example.com returned unexpected status 500", errWithoutBody.Error())
	})

	t.Run("recognizes apiError values", func(t *testing.T) {
		errWithBody := &apiError{Method: http.MethodGet, URL: "https://cas.example.com", StatusCode: 500, Body: "boom"}
		var extracted *apiError

		assert.True(t, errorAs(errWithBody, &extracted))
		assert.Same(t, errWithBody, extracted)
		assert.False(t, errorAs(errors.New("other"), &extracted))
	})
}

func newClientForHTTPServerTest(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	client, err := NewClient(ClientConfig{
		BaseURL:  server.URL,
		Username: "cas-user",
		Password: "cas-password",
		Timeout:  time.Second,
	}, server.Client())
	require.NoError(t, err)

	return client
}
