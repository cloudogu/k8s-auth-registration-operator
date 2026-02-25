package config

import (
	"os"
	"testing"

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
		assert.NotNil(t, actual.ControllerOptions)
	})
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "test log level not set",
			wantErr: assert.Error,
		},
		{
			name:    "test log level set to debug",
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
