package rancher2

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression test for #2654: updating a rancher2_cluster_v2 must not clear
// metadata.finalizers that Terraform does not manage, otherwise Rancher
// removes the field.cattle.io/creatorId annotation on specless updates.
func TestResourceRancher2ClusterV2UpdatePreservesFinalizers(t *testing.T) {
	const (
		id         = "fleet-default/test"
		creatorKey = "field.cattle.io/creatorId"
	)
	finalizers := []string{"wrangler.cattle.io/provisioning-cluster-remove"}

	var (
		mu      sync.Mutex
		putBody map[string]interface{}
	)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":    id,
				"type":  clusterV2APIType,
				"links": map[string]string{"self": srv.URL + "/" + id},
				"metadata": map[string]interface{}{
					"name":            "test",
					"namespace":       "fleet-default",
					"resourceVersion": "1",
					"finalizers":      finalizers,
					"annotations":     map[string]string{creatorKey: "user-abc"},
				},
				"spec": map[string]interface{}{"kubernetesVersion": "v1.36.6+rke2r1"},
			})
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			_ = json.Unmarshal(body, &putBody)
			mu.Unlock()
			_, _ = w.Write(body)
		}
	}))
	defer srv.Close()

	opts := &clientbase.ClientOpts{URL: srv.URL, Timeout: 5 * time.Second}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	cli := &clientbase.APIBaseClient{
		Opts: opts,
		Ops: &clientbase.APIOperations{
			Opts:   opts,
			Client: httpClient,
		},
		Types: map[string]types.Schema{
			clusterV2APIType: {
				ID:              clusterV2APIType,
				ResourceMethods: []string{"GET", "PUT"},
				Links:           map[string]string{"collection": srv.URL},
			},
		},
	}
	cli.Ops.Types = cli.Types

	cfg := &Config{
		Timeout: 5 * time.Second,
		Client: Client{
			CatalogV2: map[string]*clientbase.APIBaseClient{rancher2DefaultLocalClusterID: cli},
		},
	}

	d := schema.TestResourceDataRaw(t, clusterV2Fields(), map[string]interface{}{
		"name":               "test",
		"fleet_namespace":    "fleet-default",
		"kubernetes_version": "v1.36.6+rke2r1",
		"annotations":        map[string]interface{}{creatorKey: "user-abc"},
		"labels":             map[string]interface{}{"qa-test": "1"},
	})
	d.SetId(id)

	// The post-update read needs clients this test doesn't provide; only the
	// outgoing PUT matters here.
	func() {
		defer func() { _ = recover() }()
		_ = resourceRancher2ClusterV2Update(d, cfg)
	}()

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, putBody, "expected a PUT to be sent")
	meta, ok := putBody["metadata"].(map[string]interface{})
	require.True(t, ok, "PUT body has no metadata")
	assert.Equal(t, []interface{}{finalizers[0]}, meta["finalizers"],
		"update must carry over existing metadata.finalizers")
}
