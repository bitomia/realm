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
	jobsPkg "github.com/bitomia/realm/drivers/jobs/hello"
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

func (d *failingJobDriver) Run(w common.JobResultWriter, args ...string) error {
	return fmt.Errorf("job blew up")
}

func (d *failingJobDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: failingJobDriverID, DriverConfig: nil}
}

func init() {
	// The hello driver registers itself via the hello package's init.
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

// collect runs the job and returns the results the client handed back.
func collect(client Client, job *common.Job, arguments ...string) ([]common.JobResult, error) {
	var results []common.JobResult
	err := client.RunJob(job, func(r common.JobResult) { results = append(results, r) }, arguments...)
	return results, err
}

func TestClientRunJob(t *testing.T) {
	server := newAgentServer(t)
	client := NewUnauthClient()

	results, err := collect(client, jobOn(server, "greet", &jobsPkg.HelloDriver{}))

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Value)
	assert.Equal(t, "hello world", *results[0].Value)
	assert.Nil(t, results[0].Err)
}

// The hello driver repeats itself as many times as its first argument asks for,
// and every greeting reaches the client.
func TestClientRunJobWithArguments(t *testing.T) {
	server := newAgentServer(t)
	client := NewUnauthClient()

	results, err := collect(client, jobOn(server, "greet", &jobsPkg.HelloDriver{}), "2")

	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "hello world", *results[0].Value)
	assert.Equal(t, "hello world", *results[1].Value)
}

func TestClientRunJobFailedJob(t *testing.T) {
	server := newAgentServer(t)
	client := NewUnauthClient()

	results, err := collect(client, jobOn(server, "boom", &failingJobDriver{}))

	require.NoError(t, err, "a failed job is not a transport error")
	require.Len(t, results, 1)
	assert.Nil(t, results[0].Value)
	require.NotNil(t, results[0].Err)
	assert.Equal(t, "job blew up", *results[0].Err)
}

// The request the agent receives must carry the job name and driver config
// plus the arguments given on the command line.
func TestClientRunJobRequestPayload(t *testing.T) {
	var method, path string
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		body, _ = io.ReadAll(r.Body)

		value := "ok"
		_ = json.NewEncoder(w).Encode(common.JobResult{Value: &value})
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	results, err := collect(client, jobOn(server, "greet", &jobsPkg.HelloDriver{}), "one", "two")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "ok", *results[0].Value)

	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "/jobs", path)

	var sent map[string]any
	require.NoError(t, json.Unmarshal(body, &sent))

	assert.Equal(t, []any{"one", "two"}, sent["arguments"])
	assert.Equal(t, "greet", sent["name"])
	assert.Equal(t, "hello", sent["driver"])
	assert.NotContains(t, sent, "node", "the agent runs the driver, it never resolves the node")
	assert.NotContains(t, sent, "job", "the job is no longer embedded in the request")
}

func TestClientRunJobOmitsEmptyArguments(t *testing.T) {
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	_, err := collect(client, jobOn(server, "greet", &jobsPkg.HelloDriver{}))
	require.NoError(t, err)

	var sent map[string]any
	require.NoError(t, json.Unmarshal(body, &sent))
	assert.NotContains(t, sent, "arguments")
}

func TestClientRunJobAgentError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "driver not registered", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	results, err := collect(client, jobOn(server, "greet", &jobsPkg.HelloDriver{}))

	assert.Empty(t, results)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver not registered")
}

func TestClientRunJobUnreachableAgent(t *testing.T) {
	server := newAgentServer(t)
	url := server.URL
	server.Close()

	client := NewUnauthClient()
	job := &common.Job{Name: "greet", Node: &common.Node{Name: "lab1", Url: url}, Driver: &jobsPkg.HelloDriver{}}

	results, err := collect(client, job)

	assert.Empty(t, results)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to make request")
}

func TestClientRunJobInvalidResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	results, err := collect(client, jobOn(server, "greet", &jobsPkg.HelloDriver{}))

	assert.Empty(t, results)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse job result")
}

// Results are handed over as they arrive, not buffered until the job ends.
func TestClientRunJobHandlesResultsAsTheyArrive(t *testing.T) {
	seen := make(chan string, 2)
	released := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := common.NewJobResultWriter(w)
		require.NoError(t, writer.WriteValue("first"))
		<-released
		require.NoError(t, writer.WriteValue("second"))
	}))
	t.Cleanup(server.Close)

	client := NewUnauthClient()
	done := make(chan error, 1)
	go func() {
		done <- client.RunJob(jobOn(server, "greet", &jobsPkg.HelloDriver{}), func(r common.JobResult) {
			seen <- *r.Value
		})
	}()

	assert.Equal(t, "first", <-seen, "the first result arrives before the job finishes")
	close(released)
	assert.Equal(t, "second", <-seen)
	require.NoError(t, <-done)
}
