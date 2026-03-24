package config

import (
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewOperatorConfig(t *testing.T) {
	testScheme := runtime.NewScheme()

	t.Run("should use development stage and fail to get namespace", func(t *testing.T) {
		// given
		t.Setenv(StageEnvVar, StageDevelopment)
		t.Setenv(namespaceEnvVar, "")
		err := os.Unsetenv(namespaceEnvVar)
		require.NoError(t, err)

		oldLog := log
		defer func() { log = oldLog }()
		logMock := newMockLogSink(t)
		logMock.EXPECT().Init(mock.Anything).Return()
		logMock.EXPECT().Enabled(0).Return(true)
		logMock.EXPECT().Info(0, "Starting in development mode! This is not recommended for production!").Return()
		log = logr.New(logMock)

		// when
		actual, err := NewOperatorConfig(testScheme)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to read namespace: failed to get env var [NAMESPACE]: environment variable NAMESPACE must be set")
		assert.Nil(t, actual)
	})
	t.Run("should use development stage and succeed", func(t *testing.T) {
		// given
		t.Setenv(StageEnvVar, StageDevelopment)
		t.Setenv(namespaceEnvVar, "ecosystem")
		t.Setenv(casBaseURLEnvVar, "https://cas.example.com/cas")
		t.Setenv(casUsernameEnvVar, "cas-user")
		t.Setenv(casPasswordEnvVar, "cas-password")
		t.Setenv(casTimeoutEnvVar, "15s")

		oldLog := log
		defer func() { log = oldLog }()
		logMock := newMockLogSink(t)
		logMock.EXPECT().Init(mock.Anything).Return()
		logMock.EXPECT().Enabled(0).Return(true)
		logMock.EXPECT().Info(0, "Starting in development mode! This is not recommended for production!").Return()
		logMock.EXPECT().Info(0, "Deploying the k8s-auth-registration-operator in namespace ecosystem").Return()
		log = logr.New(logMock)

		// when
		actual, err := NewOperatorConfig(testScheme)

		// then
		require.NoError(t, err)
		assert.Equal(t, "ecosystem", actual.Namespace)
		assert.Equal(t, "https://cas.example.com/cas", actual.Cas.BaseURL)
		assert.Equal(t, "cas-user", actual.Cas.Username)
		assert.Equal(t, "cas-password", actual.Cas.Password)
		assert.Equal(t, 15*time.Second, actual.Cas.Timeout)
		assert.NotNil(t, actual.ControllerOptions)
	})

	t.Run("should fail when Cas config cannot be read", func(t *testing.T) {
		t.Setenv(StageEnvVar, StageProduction)
		t.Setenv(namespaceEnvVar, "ecosystem")
		t.Setenv(casBaseURLEnvVar, "https://cas.example.com/cas")
		t.Setenv(casUsernameEnvVar, "cas-user")
		t.Setenv(casPasswordEnvVar, "cas-password")
		t.Setenv(casTimeoutEnvVar, "invalid-duration")

		oldLog := log
		defer func() { log = oldLog }()
		logMock := newMockLogSink(t)
		logMock.EXPECT().Init(mock.Anything).Return()
		logMock.EXPECT().Enabled(0).Return(true)
		logMock.EXPECT().Info(0, "Deploying the k8s-auth-registration-operator in namespace ecosystem").Return()
		log = logr.New(logMock)

		actual, err := NewOperatorConfig(testScheme)

		require.Error(t, err)
		assert.Nil(t, actual)
		assert.ErrorContains(t, err, "failed to read Cas config")
		assert.ErrorContains(t, err, "failed to parse env var [CAS_TIMEOUT]")
	})
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "returns error when LOG_LEVEL is not set",
			wantErr: assert.Error,
		},
		{
			name:    "returns configured value when LOG_LEVEL is set",
			want:    "debug",
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want != "" {
				t.Setenv(logLevelEnvVar, tt.want)
			} else {
				// first set it so it got rolled back afterward
				t.Setenv(logLevelEnvVar, "")
				// then unset it, so environments with this envVar also work with this test
				err := os.Unsetenv(logLevelEnvVar)
				if err != nil {
					require.NoError(t, err)
				}
			}
			got, err := GetLogLevel()
			if !tt.wantErr(t, err, "GetLogLevel()") {
				return
			}
			assert.Equalf(t, tt.want, got, "GetLogLevel()")
		})
	}
}

func TestConfigureStage(t *testing.T) {
	t.Run("should set stage to development and log development warning", func(t *testing.T) {
		t.Setenv(StageEnvVar, StageDevelopment)

		oldStage := Stage
		oldLog := log
		defer func() {
			Stage = oldStage
			log = oldLog
		}()

		logMock := newMockLogSink(t)
		logMock.EXPECT().Init(mock.Anything).Return()
		logMock.EXPECT().Enabled(mock.Anything).Return(true).Maybe()
		logMock.EXPECT().Info(0, "Starting in development mode! This is not recommended for production!").Return()
		log = logr.New(logMock)

		configureStage()

		assert.Equal(t, StageDevelopment, Stage)
	})

	t.Run("should set stage to production when configured as production", func(t *testing.T) {
		t.Setenv(StageEnvVar, StageProduction)

		oldStage := Stage
		oldLog := log
		defer func() {
			Stage = oldStage
			log = oldLog
		}()

		logMock := newMockLogSink(t)
		logMock.EXPECT().Init(mock.Anything).Return()
		logMock.EXPECT().Enabled(mock.Anything).Return(true).Maybe()
		log = logr.New(logMock)

		configureStage()

		assert.Equal(t, StageProduction, Stage)
	})

	t.Run("should fall back to production and log error when stage env is missing", func(t *testing.T) {
		t.Setenv(StageEnvVar, "")
		err := os.Unsetenv(StageEnvVar)
		require.NoError(t, err)

		oldStage := Stage
		oldLog := log
		defer func() {
			Stage = oldStage
			log = oldLog
		}()

		logMock := newMockLogSink(t)
		logMock.EXPECT().Init(mock.Anything).Return()
		logMock.EXPECT().Enabled(mock.Anything).Return(true).Maybe()
		logMock.EXPECT().Error(mock.Anything, "Error reading stage environment variable. Use stage production").Return()
		log = logr.New(logMock)

		configureStage()

		assert.Equal(t, StageProduction, Stage)
	})
}

func TestGetCASConfig(t *testing.T) {
	t.Run("returns error when required Cas env vars are missing", func(t *testing.T) {
		t.Setenv(casBaseURLEnvVar, "")
		require.NoError(t, os.Unsetenv(casBaseURLEnvVar))

		_, err := getCASConfig()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get env var [CAS_BASE_URL]")
	})

	t.Run("returns error when Cas username is missing", func(t *testing.T) {
		t.Setenv(casBaseURLEnvVar, "https://cas.example.com/cas")
		t.Setenv(casUsernameEnvVar, "")
		require.NoError(t, os.Unsetenv(casUsernameEnvVar))
		t.Setenv(casPasswordEnvVar, "cas-password")

		_, err := getCASConfig()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get env var [CAS_USERNAME]")
	})

	t.Run("returns error when Cas password is missing", func(t *testing.T) {
		t.Setenv(casBaseURLEnvVar, "https://cas.example.com/cas")
		t.Setenv(casUsernameEnvVar, "cas-user")
		t.Setenv(casPasswordEnvVar, "")
		require.NoError(t, os.Unsetenv(casPasswordEnvVar))

		_, err := getCASConfig()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get env var [CAS_PASSWORD]")
	})

	t.Run("uses default timeout when CAS_TIMEOUT is not set", func(t *testing.T) {
		t.Setenv(casBaseURLEnvVar, "https://cas.example.com/cas")
		t.Setenv(casUsernameEnvVar, "cas-user")
		t.Setenv(casPasswordEnvVar, "cas-password")
		t.Setenv(casTimeoutEnvVar, "")
		require.NoError(t, os.Unsetenv(casTimeoutEnvVar))

		cfg, err := getCASConfig()

		require.NoError(t, err)
		assert.Equal(t, defaultCASTimeout, cfg.Timeout)
	})

	t.Run("returns error when Cas timeout cannot be parsed", func(t *testing.T) {
		t.Setenv(casBaseURLEnvVar, "https://cas.example.com/cas")
		t.Setenv(casUsernameEnvVar, "cas-user")
		t.Setenv(casPasswordEnvVar, "cas-password")
		t.Setenv(casTimeoutEnvVar, "not-a-duration")

		_, err := getCASConfig()

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse env var [CAS_TIMEOUT]")
	})
}

func TestGetEnvDuration(t *testing.T) {
	t.Run("returns configured duration when env var is valid", func(t *testing.T) {
		t.Setenv(casTimeoutEnvVar, "30s")

		duration, err := getEnvDuration(casTimeoutEnvVar, defaultCASTimeout)

		require.NoError(t, err)
		assert.Equal(t, 30*time.Second, duration)
	})

	t.Run("returns error when env var cannot be parsed", func(t *testing.T) {
		t.Setenv(casTimeoutEnvVar, "broken")

		duration, err := getEnvDuration(casTimeoutEnvVar, defaultCASTimeout)

		require.Error(t, err)
		assert.Equal(t, time.Duration(0), duration)
	})
}
