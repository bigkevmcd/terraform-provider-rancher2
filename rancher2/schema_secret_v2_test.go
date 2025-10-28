package rancher2

import (
	"encoding/json"
	"testing"

	norman "github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The SecretV2 struct warns that it has two `json:"type"` declarations.
//
// One is on the corev1.Secret and the other on the norman.Resource.
//
// The expected result is not clear.
func TestSecretV2JSON(t *testing.T) {
	s1 := SecretV2{
		Secret: corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testing",
				Namespace: "test-ns",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"testing": []byte("value"),
			},
		},
		Resource: norman.Resource{
			ID:   "testing",
			Type: "secret",
			Links: map[string]string{
				"remove": "https://localhost:9443/v3/secrets/test-ns/testing",
				"self":   "https://localhost:9443/v3/secrets/test-ns/testing",
				"update": "https://localhost:9443/v3/secrets/test-ns/testing",
			},
			Actions: map[string]string{},
		},
		K8SType: "Secret",
	}

	b, err := json.Marshal(s1)
	assert.NoError(t, err)

	var s2 SecretV2
	assert.NoError(t, json.Unmarshal(b, &s2))

	assert.Equal(t, s1, s2)

	var s3 map[string]any
	assert.NoError(t, json.Unmarshal(b, &s3))

	// Loses the Opaque part of the Secret.
	// Probably not that important in the browser response.
	want := map[string]any{
		"_type":   "Secret",
		"actions": map[string]any{},
		"data": map[string]any{
			"testing": "dmFsdWU=",
		},
		"id": "testing",
		"links": map[string]any{
			"remove": "https://localhost:9443/v3/secrets/test-ns/testing",
			"self":   "https://localhost:9443/v3/secrets/test-ns/testing",
			"update": "https://localhost:9443/v3/secrets/test-ns/testing",
		},
		"metadata": map[string]any{
			"name":      "testing",
			"namespace": "test-ns",
		},
	}

	assert.Equal(t, want, s3)
}
