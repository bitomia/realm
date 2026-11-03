package golang

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/bitomia/realm/common"
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
