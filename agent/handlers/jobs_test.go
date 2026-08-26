package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common"
)

const handlerJobDriverID common.JobDriverID = "handler-test"

// handlerJobDriver is rebuilt per request from the driver_config carried in the
// request body:
//
//	{"reply": "hola"}  -> Run streams "hola[args...]"
//	{"fail": "boom"}   -> Run returns an error
//	{"times": 3}       -> Run streams the reply that many times
type handlerJobDriver struct {
	reply string
	fail  string
	times int
}

func (d *handlerJobDriver) ID() common.JobDriverID { return handlerJobDriverID }

func (d *handlerJobDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID: handlerJobDriverID,
		New: func(config any) (common.JobDriver, error) {
			driver := &handlerJobDriver{times: 1}
			if settings, ok := config.(map[string]any); ok {
				if reply, ok := settings["reply"].(string); ok {
					driver.reply = reply
				}
				if fail, ok := settings["fail"].(string); ok {
					driver.fail = fail
				}
				if times, ok := settings["times"].(float64); ok {
					driver.times = int(times)
				}
			}
			return driver, nil
		},
	}
}

func (d *handlerJobDriver) Run(w common.JobResultWriter, args ...string) error {
	if d.fail != "" {
		return fmt.Errorf("%s", d.fail)
	}
	if d.reply == "" {
		return nil
	}

	value := d.reply
	if len(args) > 0 {
		value = fmt.Sprintf("%s[%s]", d.reply, strings.Join(args, ","))
	}
	for range d.times {
		if err := w.WriteValue(value); err != nil {
			return err
		}
	}
	return nil
}

func (d *handlerJobDriver) Config() common.JobDriverConfig {
	return common.JobDriverConfig{Driver: handlerJobDriverID, DriverConfig: nil}
}

func init() {
	if err := common.RegisterJobDriver(&handlerJobDriver{}); err != nil {
		panic(err)
	}
}

func postJob(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/jobs", strings.NewReader(body))
	rec := httptest.NewRecorder()
	RunJobHandler(rec, req)
	return rec
}

func decodeJobResults(t *testing.T, rec *httptest.ResponseRecorder) []common.JobResult {
	t.Helper()

	var results []common.JobResult
	decoder := json.NewDecoder(rec.Body)
	for decoder.More() {
		var result common.JobResult
		require.NoError(t, decoder.Decode(&result))
		results = append(results, result)
	}
	return results
}

func TestRunJobHandlerSuccess(t *testing.T) {
	rec := postJob(t, `{"name":"greet","driver":"handler-test","driver_config":{"reply":"hola"}}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/x-ndjson", rec.Header().Get("Content-Type"))

	results := decodeJobResults(t, rec)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].Value)
	assert.Equal(t, "hola", *results[0].Value)
	assert.Nil(t, results[0].Err)
}

// Every result the driver produces is a separate JSON object in the response.
func TestRunJobHandlerStreamsResults(t *testing.T) {
	rec := postJob(t, `{"name":"greet","driver":"handler-test","driver_config":{"reply":"hola","times":3}}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decodeJobResults(t, rec), 3)
}

func TestRunJobHandlerForwardsArguments(t *testing.T) {
	rec := postJob(t, `{"name":"greet","driver":"handler-test","driver_config":{"reply":"hola"},"arguments":["one","two"]}`)

	require.Equal(t, http.StatusOK, rec.Code)

	results := decodeJobResults(t, rec)
	require.Len(t, results, 1)
	assert.Equal(t, "hola[one,two]", *results[0].Value)
}

func TestRunJobHandlerDriverFailure(t *testing.T) {
	rec := postJob(t, `{"name":"greet","driver":"handler-test","driver_config":{"fail":"boom"}}`)

	require.Equal(t, http.StatusOK, rec.Code, "a failing job is a successful request")

	results := decodeJobResults(t, rec)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].Value)
	require.NotNil(t, results[0].Err)
	assert.Equal(t, "boom", *results[0].Err)
}

func TestRunJobHandlerEmptyResult(t *testing.T) {
	rec := postJob(t, `{"name":"greet","driver":"handler-test"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String(), "a job that produces nothing streams nothing")
}

// A request without a driver cannot be built, and nothing has been streamed
// yet, so the handler answers with a status code.
func TestRunJobHandlerMissingDriver(t *testing.T) {
	rec := postJob(t, `{"arguments":["one"]}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_registered")
}

func TestRunJobHandlerMalformedBody(t *testing.T) {
	rec := postJob(t, `{"name":`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRunJobHandlerEmptyBody(t *testing.T) {
	rec := postJob(t, ``)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "EOF")
}

func TestRunJobHandlerUnknownDriver(t *testing.T) {
	rec := postJob(t, `{"name":"greet","driver":"nosuchdriver"}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "nosuchdriver")
	assert.Contains(t, rec.Body.String(), "not_registered")
}

func TestRunJobHandlerRebuildsDriverPerRequest(t *testing.T) {
	first := postJob(t, `{"name":"greet","driver":"handler-test","driver_config":{"reply":"hola"}}`)
	second := postJob(t, `{"name":"greet","driver":"handler-test","driver_config":{"reply":"adeu"}}`)

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)

	assert.Equal(t, "hola", *decodeJobResults(t, first)[0].Value)
	assert.Equal(t, "adeu", *decodeJobResults(t, second)[0].Value)
}
