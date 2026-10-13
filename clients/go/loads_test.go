package golang

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

func TestClientStartLoadsRunsEachLoadOnce(t *testing.T) {
	var started []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /loads/{name}/start", func(w http.ResponseWriter, r *http.Request) {
		started = append(started, r.PathValue("name"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	node := &common.Node{Name: "lab1", Url: server.URL}
	db := &common.Load{Name: "db", Node: node}
	db.StartChain = common.LoadChain{db}
	api := &common.Load{Name: "api", Node: node, DependsOn: []*common.Load{db}}
	api.StartChain = common.LoadChain{db, api}

	client := newTestClient()
	var handled []string
	client.StartLoads(map[string]*common.Load{"db": db, "api": api}, func(load *common.Load, err error) {
		assert.NoError(t, err)
		handled = append(handled, load.Name)
	})

	assert.ElementsMatch(t, []string{"db", "api"}, started)
	assert.Equal(t, started, handled)
	// The dependency is always started before its dependent.
	assert.Equal(t, "db", started[0])
}

func TestClientGetLoadsStates(t *testing.T) {
	var requests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /loads", func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode(dto.LoadsDeployments{{
			LoadName:         "web",
			Driver:           "container",
			DeploymentStatus: common.DeploymentStatus{StatusCode: common.DeploymentStatusRunning},
			Metadata:         map[string]any{"container_name": "web-1"},
		}})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(failing.Close)

	node := &common.Node{Name: "lab1", Url: server.URL}
	loads := map[string]*common.Load{
		"web":    {Name: "web", Node: node},
		"worker": {Name: "worker", Node: node},
		"broken": {Name: "broken", Node: &common.Node{Name: "lab2", Url: failing.URL}},
	}

	client := newTestClient()
	states := client.GetLoadsStates(loads)
	require.Len(t, states, 3)

	// Sorted by load name.
	broken, web, worker := states[0], states[1], states[2]
	assert.Equal(t, "broken", broken.Load.Name)
	assert.Error(t, broken.Err)
	assert.False(t, broken.Provisioned)

	assert.NoError(t, web.Err)
	assert.True(t, web.Provisioned)
	assert.Equal(t, common.DeploymentStatusRunning, web.Status.StatusCode)
	assert.Equal(t, []string{"web-1"}, web.ContainerNames)

	assert.NoError(t, worker.Err)
	assert.False(t, worker.Provisioned)

	// Each node is queried once.
	assert.Equal(t, int32(1), requests.Load())
}

func TestClientStreamLoadStdout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /loads/web/stdout", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello\n"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := newTestClient()
	var out strings.Builder
	err := client.StreamLoadStdout(&common.Load{Name: "web", Node: &common.Node{Url: server.URL}}, &out)
	require.NoError(t, err)
	assert.Equal(t, "hello\n", out.String())
}
