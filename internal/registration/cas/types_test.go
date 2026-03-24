package cas

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisteredServicePropertiesMarshalAndUnmarshal(t *testing.T) {
	properties := NewRegisteredServiceProperties()
	properties.Entries["ServiceName"] = NewRegisteredServiceProperty("app")

	payload, err := json.Marshal(properties)
	require.NoError(t, err)
	assert.Contains(t, string(payload), `"@class":"java.util.HashMap"`)
	assert.Contains(t, string(payload), `"ServiceName"`)

	var decoded RegisteredServiceProperties
	err = json.Unmarshal(payload, &decoded)

	require.NoError(t, err)
	assert.Equal(t, hashMapClass, decoded.Class)
	assert.Equal(t, "app", decoded.GetFirstValue("ServiceName"))
}

func TestRegisteredServicePropertiesUnmarshalError(t *testing.T) {
	var decoded RegisteredServiceProperties

	err := json.Unmarshal([]byte(`{"ServiceName":"broken"}`), &decoded)

	require.Error(t, err)
	assert.ErrorContains(t, err, `failed to unmarshal property "ServiceName"`)
}

func TestRegisteredServicePropertiesGetFirstValueHandlesMissingData(t *testing.T) {
	var decoded RegisteredServiceProperties
	assert.Equal(t, "", decoded.GetFirstValue("missing"))

	var nilProperties *RegisteredServiceProperties
	assert.Equal(t, "", nilProperties.GetFirstValue("missing"))
}

func TestStringCollectionMarshalAndUnmarshal(t *testing.T) {
	t.Run("marshals with default hash set class", func(t *testing.T) {
		collection := NewStringCollection("", "openid", "profile")

		payload, err := json.Marshal(collection)

		require.NoError(t, err)
		assert.Equal(t, `["java.util.HashSet",["openid","profile"]]`, string(payload))
	})

	t.Run("unmarshals wrapper payload", func(t *testing.T) {
		var decodedWrapper StringCollection

		err := json.Unmarshal([]byte(`["java.util.LinkedHashSet",["openid","profile"]]`), &decodedWrapper)

		require.NoError(t, err)
		assert.Equal(t, linkedHashSetClass, decodedWrapper.Class)
		assert.Equal(t, []string{"openid", "profile"}, decodedWrapper.Values)
	})

	t.Run("unmarshals direct array payload", func(t *testing.T) {
		var decodedDirect StringCollection

		err := json.Unmarshal([]byte(`["openid"]`), &decodedDirect)

		require.NoError(t, err)
		assert.Equal(t, hashSetClass, decodedDirect.Class)
		assert.Equal(t, []string{"openid"}, decodedDirect.Values)
	})

	t.Run("returns error for ambiguous two-element array", func(t *testing.T) {
		var decodedAmbiguous StringCollection

		err := json.Unmarshal([]byte(`["openid","profile"]`), &decodedAmbiguous)

		require.Error(t, err)
	})

	t.Run("handles null payload", func(t *testing.T) {
		var decodedNull StringCollection

		err := json.Unmarshal([]byte(`null`), &decodedNull)

		require.NoError(t, err)
		assert.Empty(t, decodedNull)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		var decodedInvalid StringCollection

		err := json.Unmarshal([]byte(`123`), &decodedInvalid)

		require.Error(t, err)
	})
}

func TestRegisteredServicesListUnmarshal(t *testing.T) {
	t.Run("unmarshals wrapper payload", func(t *testing.T) {
		var wrapperList registeredServicesList

		err := json.Unmarshal([]byte(`["java.util.ArrayList",[{"id":1,"name":"app"}]]`), &wrapperList)

		require.NoError(t, err)
		require.Len(t, wrapperList, 1)
		assert.Equal(t, int64(1), wrapperList[0].ID)
	})

	t.Run("unmarshals object payload", func(t *testing.T) {
		var objectList registeredServicesList

		err := json.Unmarshal([]byte(`{"services":[{"id":2,"name":"app2"}]}`), &objectList)

		require.NoError(t, err)
		require.Len(t, objectList, 1)
		assert.Equal(t, int64(2), objectList[0].ID)
	})

	t.Run("unmarshals direct array payload", func(t *testing.T) {
		var directList registeredServicesList

		err := json.Unmarshal([]byte(`[{"id":3,"name":"app3"}]`), &directList)

		require.NoError(t, err)
		require.Len(t, directList, 1)
		assert.Equal(t, int64(3), directList[0].ID)
	})

	t.Run("handles null payload", func(t *testing.T) {
		var nullList registeredServicesList

		err := json.Unmarshal([]byte(`null`), &nullList)

		require.NoError(t, err)
		assert.Nil(t, nullList)
	})

	t.Run("returns error for invalid payload", func(t *testing.T) {
		var invalidList registeredServicesList

		err := json.Unmarshal([]byte(`123`), &invalidList)

		require.Error(t, err)
	})
}
