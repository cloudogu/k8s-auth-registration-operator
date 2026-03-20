package cas

import (
	"context"
	"encoding/base64"
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
		w.Header().Set("Content-Type", "application/json")
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
	_, err := NewClient(ClientConfig{}, nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "base URL must not be empty")
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
