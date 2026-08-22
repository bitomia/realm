package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/agent/handlers"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/dto"
	jobsPkg "github.com/bitomia/realm/drivers/jobs"
)

const failingJobDriverID common.JobDriverID = "client-failing"

// failingJobDriver always fails, to exercise the job-failed path end to end.
type failingJobDriver struct{}

func (d *failingJobDriver) ID() common.JobDriverID { return failingJobDriverID }

func (d *failingJobDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID:  failingJobDriverID,
		New: func(config any) (common.JobDriver, error) { return &failingJobDriver{}, nil },
	}
}

func (d *failingJobDriver) Run(args ...string) (*string, error) {
	return nil, fmt.Errorf("job blew up")
}

func (d *failingJobDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: failingJobDriverID, DriverConfig: nil}
}

func init() {
	if err := common.RegisterJobDriver(&jobsPkg.HelloDriver{}); err != nil {
		panic(err)
	}
	if err := common.RegisterJobDriver(&failingJobDriver{}); err != nil {
		panic(err)
	}
}

// newAgentServer starts an HTTP server exposing the agent's /jobs endpoint.
func newAgentServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", handlers.RunJobHandler)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func jobOn(server *httptest.Server, name string, driver common.JobDriver) *common.Job {
	return &common.Job{
		Name:   name,
		Node:   &common.Node{Name: "lab1", Url: server.URL},
		Driver: driver,
	}
}

func TestClientRunJob(t *testing.T) {
	server := newAgentServer(t)
	client := NewUnauthClient()

	result, err := client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}))

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Value)
	assert.Equal(t, "hello world", *result.Value)
	assert.Nil(t, result.Err)
}

func TestClientRunJobWithArguments(t *testing.T) {
	server := newAgentServer(t)
	client := NewUnauthClient()

	result, err := client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}), "one", "two")

	require.NoError(t, err)
	require.NotNil(t, result.Value)
	assert.Equal(t, "hello world", *result.Value, "the hello driver ignores its arguments")
}

func TestClientRunJobFailedJob(t *testing.T) {
	server := newAgentServer(t)
	client := NewUnauthClient()

	result, err := client.RunJob(jobOn(server, "boom", &failingJobDriver{}))

	require.NoError(t, err, "a failed job is not a transport error")
	require.NotNil(t, result)
	assert.Nil(t, result.Value)
	require.NotNil(t, result.Err)
	assert.Equal(t, "job blew up", *result.Err)
}

// The request the agent receives must carry the job serialized by name/driver
// plus the arguments given on the command line.
func TestClientRunJobRequestPayload(t *testing.T) {
	var method, path string
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, _ = io.ReadAll(r.Body)

		value := "ok"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dto.JobResult{Value: &value})
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	result, err := client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}), "one", "two")
	require.NoError(t, err)
	assert.Equal(t, "ok", *result.Value)

	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/jobs", path)

	var sent map[string]any
	require.NoError(t, json.Unmarshal(body, &sent))

	assert.Equal(t, []any{"one", "two"}, sent["arguments"])
	job, ok := sent["job"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "greet", job["name"])
	assert.Equal(t, "lab1", job["node"])
	assert.Equal(t, "hello", job["driver"])
}

func TestClientRunJobOmitsEmptyArguments(t *testing.T) {
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	_, err := client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}))
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, json.Unmarshal(body, &sent))
	assert.NotContains(t, sent, "arguments")
}

func TestClientRunJobAgentError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Job cannot be nil on request", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	result, err := client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}))

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Job cannot be nil on request")
}

func TestClientRunJobUnreachableAgent(t *testing.T) {
	server := newAgentServer(t)
	url := server.URL
	server.Close()

	client := NewUnauthClient()
	job := &common.Job{Name: "greet", Node: &common.Node{Name: "lab1", Url: url}, Driver: &jobsPkg.HelloDriver{}}

	result, err := client.RunJob(job)

	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to make request")
}

func TestClientRunJobInvalidResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	result, err := client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}))

	assert.Nil(t, result)
	assert.Error(t, err)
}
