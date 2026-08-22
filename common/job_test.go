package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJob(name, nodeName string, driverID JobDriverID, driverConfig any) *Job {
	return &Job{
		Name:   name,
		Node:   &Node{Name: nodeName},
		Driver: &testJobDriver{id: driverID, config: driverConfig},
	}
}

func TestJobMarshalJSON(t *testing.T) {
	job := newTestJob("greet", "lab1", "hello", map[string]any{"greeting": "hola"})

	data, err := json.Marshal(job)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, "greet", decoded["name"])
	assert.Equal(t, "lab1", decoded["node"], "the node is serialized by name, not embedded")
	assert.Equal(t, "hello", decoded["driver"])
	assert.Equal(t, map[string]any{"greeting": "hola"}, decoded["driver_config"])
}

func TestJobMarshalJSONNilDriverConfig(t *testing.T) {
	job := newTestJob("greet", "lab1", "hello", nil)

	data, err := json.Marshal(job)
	require.NoError(t, err)

	assert.JSONEq(t, `{"name":"greet","node":"lab1","driver":"hello","driver_config":null}`, string(data))
}

func TestJobUnmarshalJSON(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "job-unmarshal"})

	payload := `{"name":"greet","node":"lab1","driver":"job-unmarshal","driver_config":{"greeting":"hola"}}`

	var job Job
	require.NoError(t, json.Unmarshal([]byte(payload), &job))

	assert.Equal(t, "greet", job.Name)
	require.NotNil(t, job.Driver)
	assert.Equal(t, JobDriverID("job-unmarshal"), job.Driver.ID())
	assert.Equal(t, map[string]any{"greeting": "hola"}, job.Driver.Config().DriverConfig,
		"the driver must be rebuilt from the serialized driver_config")
}

// The agent rebuilds the driver from the payload but does not resolve the node
// reference: `Job.Node` stays nil after decoding. api.RunJob only needs the
// driver, so this is fine for the request path, but a decoded job cannot be
// re-marshaled or hashed (both dereference Node) -- see
// TestJobDecodedOnTheWireCannotBeReMarshaled.
func TestJobUnmarshalJSONDoesNotResolveNode(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "job-unmarshal-node"})

	payload := `{"name":"greet","node":"lab1","driver":"job-unmarshal-node"}`

	var job Job
	require.NoError(t, json.Unmarshal([]byte(payload), &job))

	assert.Nil(t, job.Node)
}

func TestJobDecodedOnTheWireCannotBeReMarshaled(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "job-remarshal"})

	var job Job
	require.NoError(t, json.Unmarshal([]byte(`{"name":"greet","node":"lab1","driver":"job-remarshal"}`), &job))

	assert.Panics(t, func() { _, _ = json.Marshal(&job) }, "nil Node is dereferenced by MarshalJSON")
	assert.Panics(t, func() { job.Hash() }, "nil Node is dereferenced by Hash")
}

func TestJobUnmarshalJSONUnknownDriver(t *testing.T) {
	var job Job
	err := json.Unmarshal([]byte(`{"name":"greet","node":"lab1","driver":"job-not-registered"}`), &job)

	require.Error(t, err)
	assert.Contains(t, err.Error(), string(JobDriverErrNotRegistered))
}

func TestJobUnmarshalJSONDriverBuildError(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "job-buildfail", buildErr: assert.AnError})

	var job Job
	err := json.Unmarshal([]byte(`{"name":"greet","node":"lab1","driver":"job-buildfail"}`), &job)

	require.Error(t, err)
	assert.Contains(t, err.Error(), string(JobDriverErrBuildFailed))
}

func TestJobUnmarshalJSONMalformedPayload(t *testing.T) {
	var job Job
	require.Error(t, json.Unmarshal([]byte(`{"name":`), &job))
}

func TestJobRoundTrip(t *testing.T) {
	registerTestJobDriver(t, &testJobDriver{id: "job-roundtrip"})

	original := &Job{
		Name:   "greet",
		Node:   &Node{Name: "lab1"},
		Driver: &testJobDriver{id: "job-roundtrip", config: map[string]any{"greeting": "hola"}},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Job
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Driver.ID(), decoded.Driver.ID())
	assert.Equal(t, original.Driver.Config().DriverConfig, decoded.Driver.Config().DriverConfig)
}

func TestJobHashIsDeterministic(t *testing.T) {
	job := newTestJob("greet", "lab1", "hello", map[string]any{"greeting": "hola"})

	first := job.Hash()
	second := job.Hash()

	assert.Equal(t, first, second)
	assert.NotEqual(t, [32]byte{}, first)
}

func TestJobHashUniqueness(t *testing.T) {
	base := newTestJob("greet", "lab1", "hello", map[string]any{"greeting": "hola"})

	otherName := newTestJob("salute", "lab1", "hello", map[string]any{"greeting": "hola"})
	otherNode := newTestJob("greet", "lab2", "hello", map[string]any{"greeting": "hola"})
	otherDriver := newTestJob("greet", "lab1", "bye", map[string]any{"greeting": "hola"})
	otherConfig := newTestJob("greet", "lab1", "hello", map[string]any{"greeting": "adeu"})
	nilConfig := newTestJob("greet", "lab1", "hello", nil)

	baseHash := base.Hash()

	assert.NotEqual(t, baseHash, otherName.Hash(), "different names must hash differently")
	assert.NotEqual(t, baseHash, otherNode.Hash(), "different nodes must hash differently")
	assert.NotEqual(t, baseHash, otherDriver.Hash(), "different drivers must hash differently")
	assert.NotEqual(t, baseHash, otherConfig.Hash(), "different driver configs must hash differently")
	assert.NotEqual(t, baseHash, nilConfig.Hash(), "a nil driver config must hash differently")
}

func TestJobHashIgnoresDriverConfigFieldOrder(t *testing.T) {
	first := newTestJob("greet", "lab1", "hello", map[string]any{"greeting": "hola", "times": 2})
	second := newTestJob("greet", "lab1", "hello", map[string]any{"times": 2, "greeting": "hola"})

	assert.Equal(t, first.Hash(), second.Hash())
}
