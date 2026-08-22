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
	"github.com/bitomia/realm/common/dto"
)

const handlerJobDriverID common.JobDriverID = "handler-test"

// handlerJobDriver is rebuilt by common.Job.UnmarshalJSON from the
// driver_config carried in the request body:
//
//	{"reply": "hola"}  -> Run returns "hola[args...]"
//	{"fail": "boom"}   -> Run returns an error
type handlerJobDriver struct {
	reply string
	fail  string
}

func (d *handlerJobDriver) ID() common.JobDriverID { return handlerJobDriverID }

func (d *handlerJobDriver) Info() common.JobDriverInfo {
	return common.JobDriverInfo{
		ID: handlerJobDriverID,
		New: func(config any) (common.JobDriver, error) {
			driver := &handlerJobDriver{}
			if settings, ok := config.(map[string]any); ok {
				if reply, ok := settings["reply"].(string); ok {
					driver.reply = reply
				}
				if fail, ok := settings["fail"].(string); ok {
					driver.fail = fail
				}
			}
			return driver, nil
		},
	}
}

func (d *handlerJobDriver) Run(args ...string) (*string, error) {
	if d.fail != "" {
		return nil, fmt.Errorf("%s", d.fail)
	}
	if d.reply == "" {
		return nil, nil
	}

	value := d.reply
	if len(args) > 0 {
		value = fmt.Sprintf("%s[%s]", d.reply, strings.Join(args, ","))
	}
	return &value, nil
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

func decodeJobResult(t *testing.T, rec *httptest.ResponseRecorder) dto.JobResult {
	t.Helper()

	var result dto.JobResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	return result
}

func TestRunJobHandlerSuccess(t *testing.T) {
	rec := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"handler-test","driver_config":{"reply":"hola"}}}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	result := decodeJobResult(t, rec)
	require.NotNil(t, result.Value)
	assert.Equal(t, "hola", *result.Value)
	assert.Nil(t, result.Err)
}

func TestRunJobHandlerForwardsArguments(t *testing.T) {
	rec := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"handler-test","driver_config":{"reply":"hola"}},"arguments":["one","two"]}`)

	require.Equal(t, http.StatusOK, rec.Code)

	result := decodeJobResult(t, rec)
	require.NotNil(t, result.Value)
	assert.Equal(t, "hola[one,two]", *result.Value)
}

func TestRunJobHandlerDriverFailure(t *testing.T) {
	rec := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"handler-test","driver_config":{"fail":"boom"}}}`)

	require.Equal(t, http.StatusOK, rec.Code, "a failing job is a successful request")

	result := decodeJobResult(t, rec)
	assert.Nil(t, result.Value)
	require.NotNil(t, result.Err)
	assert.Equal(t, "boom", *result.Err)
}

func TestRunJobHandlerEmptyResult(t *testing.T) {
	rec := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"handler-test"}}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{}`, rec.Body.String(), "value and err are both omitted when the job returns nothing")
}

func TestRunJobHandlerNilJob(t *testing.T) {
	rec := postJob(t, `{"arguments":["one"]}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Job cannot be nil on request")
}

func TestRunJobHandlerExplicitNullJob(t *testing.T) {
	rec := postJob(t, `{"job":null}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "Job cannot be nil on request")
}

func TestRunJobHandlerMalformedBody(t *testing.T) {
	rec := postJob(t, `{"job":`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRunJobHandlerEmptyBody(t *testing.T) {
	rec := postJob(t, ``)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "EOF")
}

func TestRunJobHandlerUnknownDriver(t *testing.T) {
	rec := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"nosuchdriver"}}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "nosuchdriver")
	assert.Contains(t, rec.Body.String(), "not_registered")
}

func TestRunJobHandlerRebuildsDriverPerRequest(t *testing.T) {
	first := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"handler-test","driver_config":{"reply":"hola"}}}`)
	second := postJob(t, `{"job":{"name":"greet","node":"lab1","driver":"handler-test","driver_config":{"reply":"adeu"}}}`)

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, http.StatusOK, second.Code)

	assert.Equal(t, "hola", *decodeJobResult(t, first).Value)
	assert.Equal(t, "adeu", *decodeJobResult(t, second).Value)
}
