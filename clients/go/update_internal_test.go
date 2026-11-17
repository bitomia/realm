package golang

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
)

// testLoadsGraph builds db <- api <- web, plus an independent cache.
func testLoadsGraph() map[string]*common.Load {
	db := &common.Load{Name: "db"}
	db.StartChain = common.LoadChain{db}
	api := &common.Load{Name: "api", DependsOn: []*common.Load{db}}
	api.StartChain = common.LoadChain{db, api}
	web := &common.Load{Name: "web", DependsOn: []*common.Load{api}}
	web.StartChain = common.LoadChain{db, api, web}
	cache := &common.Load{Name: "cache"}
	cache.StartChain = common.LoadChain{cache}
	return map[string]*common.Load{"db": db, "api": api, "web": web, "cache": cache}
}

func names(loads []*common.Load) []string {
	result := make([]string, len(loads))
	for i, l := range loads {
		result[i] = l.Name
	}
	return result
}

func TestChangedLoads(t *testing.T) {
	current := []common.LoadConfig{
		{Name: "db", Driver: "container", DriverConfig: map[string]any{"image": "pg:16"}},
		{Name: "api", Driver: "container", DriverConfig: map[string]any{"image": "api:1"}},
		{Name: "gone", Driver: "container"},
	}
	planned := []common.LoadConfig{
		{Name: "db", Driver: "container", DriverConfig: map[string]any{"image": "pg:16"}},
		{Name: "api", Driver: "container", DriverConfig: map[string]any{"image": "api:2"}},
		{Name: "new", Driver: "container"},
	}

	assert.Equal(t, []string{"api", "new"}, changedLoads(current, planned))
	assert.Empty(t, changedLoads(planned, planned))
}

func TestDiffNodeLoads(t *testing.T) {
	load := func(name string, image string) common.LoadConfig {
		return common.LoadConfig{Name: name, Driver: "container", DriverConfig: map[string]any{"image": image}}
	}
	current := []common.LoadConfig{load("db", "pg:16"), load("api", "api:1"), load("gone", "gone:1")}
	planned := []common.LoadConfig{load("db", "pg:16"), load("api", "api:2"), load("new", "new:1")}

	stale, fresh := diffNodeLoads(current, planned, map[string]bool{"db": true, "api": true, "gone": true})
	assert.Equal(t, []string{"api", "gone"}, stale)
	assert.Equal(t, []string{"api", "new"}, fresh)

	// Planned loads missing their deployment are provisioned even when the config holds them
	stale, fresh = diffNodeLoads(planned, planned, map[string]bool{"db": true})
	assert.Empty(t, stale)
	assert.Equal(t, []string{"api", "new"}, fresh)

	stale, fresh = diffNodeLoads(planned, planned, map[string]bool{"db": true, "api": true, "new": true})
	assert.Empty(t, stale)
	assert.Empty(t, fresh)
}

func TestRolloutLoadsIncludesDependents(t *testing.T) {
	loads := testLoadsGraph()

	rollout := rolloutLoads(loads, map[string]bool{"api": true})
	assert.ElementsMatch(t, []string{"api", "web"}, slices.Collect(maps.Keys(rollout)))

	assert.Len(t, rolloutLoads(loads, map[string]bool{"db": true}), 3)
	assert.Empty(t, rolloutLoads(loads, map[string]bool{}))
}

func TestRolloutStopOrderSkipsUnchangedDependencies(t *testing.T) {
	loads := testLoadsGraph()

	rollout := rolloutLoads(loads, map[string]bool{"api": true})
	// db is an unchanged dependency: it keeps running
	assert.Equal(t, []string{"web", "api"}, names(rolloutStopOrder(rollout)))
}

// updateTestAgent records the requests of a node update and fails provisioning failLoad.
func updateTestAgent(t *testing.T, deployments dto.LoadsDeployments, failLoad string) (*common.Node, *[]string) {
	var calls []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /loads", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(deployments)
	})
	mux.HandleFunc("POST /loads/{name}/{operation}", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.PathValue("operation")+" "+r.PathValue("name"))
		if r.PathValue("operation") == "provision" && r.PathValue("name") == failLoad {
			http.Error(w, "boom", http.StatusBadRequest)
		}
	})
	mux.HandleFunc("POST /node/config", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "load config")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("DELETE /node/config", func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "unload config")
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &common.Node{Name: "lab1", Url: server.URL, Driver: &modeNodeDriver{}}, &calls
}

func TestUpdateNodeReprovisionsLoads(t *testing.T) {
	deployment := func(name string, status common.DeploymentStatusCode) dto.LoadDeployment {
		return dto.LoadDeployment{LoadName: name, DeploymentStatus: common.DeploymentStatus{StatusCode: status}}
	}
	node, calls := updateTestAgent(t, dto.LoadsDeployments{
		deployment("api", common.DeploymentStatusStopped),
		deployment("gone", common.DeploymentStatusRunning),
	}, "")

	client := newTestClient()
	err := client.updateNode(nodeUpdate{
		node:       node,
		configured: true,
		stale:      []string{"api", "gone", "vanished"},
		fresh:      []string{"api", "new"},
	})

	require.NoError(t, err)
	// Loads are provisioned out of the loaded config, once the stale ones are gone
	assert.Equal(t, []string{
		"deprovision api", "stop gone", "deprovision gone",
		"load config",
		"provision api", "provision new",
	}, *calls)
}

func TestUpdateNodeRollsBackFailedProvision(t *testing.T) {
	fresh := []string{"db", "api"}

	node, calls := updateTestAgent(t, nil, "api")
	client := newTestClient()
	err := client.updateNode(nodeUpdate{node: node, configured: true, fresh: fresh})
	assert.ErrorContains(t, err, "provisioning api failed")
	// An already configured node keeps its config
	assert.Equal(t, []string{"load config", "provision db", "provision api", "deprovision db"}, *calls)

	node, calls = updateTestAgent(t, nil, "api")
	err = client.updateNode(nodeUpdate{node: node, configured: false, fresh: fresh})
	assert.ErrorContains(t, err, "provisioning api failed")
	assert.Equal(t, []string{"load config", "provision db", "provision api", "deprovision db", "unload config"}, *calls)
}
