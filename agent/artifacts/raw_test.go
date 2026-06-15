package artifacts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bitomia/realm/common/config"
)

func setupArtifactsPath(t *testing.T, dir string) {
	t.Helper()
	prev := rawArtifactsPath
	t.Cleanup(func() { rawArtifactsPath = prev })
	if dir == "" {
		rawArtifactsPath = nil
		return
	}
	abs := dir + string(filepath.Separator)
	rawArtifactsPath = &abs
}

func newRawRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/artifacts/raw/{name}", GetRawArtifactHandler).Methods("GET")
	r.HandleFunc("/artifacts/raw", ListRawArtifactsHandler).Methods("GET")
	return r
}

func TestInitialize_NilConfig(t *testing.T) {
	router := mux.NewRouter()
	err := Initialize(nil, router)
	assert.NoError(t, err)
}

func TestInitialize_NilRouter(t *testing.T) {
	cfg := &config.ArtifactsRepository{}
	err := Initialize(cfg, nil)
	assert.Error(t, err)
}

func TestInitialize_NoRawPath(t *testing.T) {
	prev := rawArtifactsPath
	t.Cleanup(func() { rawArtifactsPath = prev })

	router := mux.NewRouter()
	cfg := &config.ArtifactsRepository{}
	err := Initialize(cfg, router)
	assert.NoError(t, err)
	assert.Equal(t, prev, rawArtifactsPath)
}

func TestInitialize_SetsRawArtifactsPath(t *testing.T) {
	prev := rawArtifactsPath
	t.Cleanup(func() { rawArtifactsPath = prev })

	dir := t.TempDir()
	cfg := &config.ArtifactsRepository{RawArtifactsPath: &dir}

	router := mux.NewRouter()
	err := Initialize(cfg, router)
	require.NoError(t, err)
	require.NotNil(t, rawArtifactsPath)

	abs, _ := filepath.Abs(dir)
	assert.Equal(t, abs+string(filepath.Separator), *rawArtifactsPath)
}

func TestInitialize_RegistersRoutes(t *testing.T) {
	prev := rawArtifactsPath
	t.Cleanup(func() { rawArtifactsPath = prev })

	dir := t.TempDir()
	content := []byte("hello")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), content, 0644))

	cfg := &config.ArtifactsRepository{RawArtifactsPath: &dir}
	router := mux.NewRouter()
	require.NoError(t, Initialize(cfg, router))

	req := httptest.NewRequest("GET", "/artifacts/raw/file.txt", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", w.Body.String())
}

func TestGetRawArtifact_NotConfigured(t *testing.T) {
	setupArtifactsPath(t, "")

	req := httptest.NewRequest("GET", "/artifacts/raw/test.txt", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestGetRawArtifact_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	content := []byte("artifact content")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "output.bin"), content, 0644))
	setupArtifactsPath(t, dir)

	req := httptest.NewRequest("GET", "/artifacts/raw/output.bin", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "artifact content", w.Body.String())
}

func TestGetRawArtifact_NotFound(t *testing.T) {
	dir := t.TempDir()
	setupArtifactsPath(t, dir)

	req := httptest.NewRequest("GET", "/artifacts/raw/missing.txt", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListRawArtifacts_NotConfigured(t *testing.T) {
	setupArtifactsPath(t, "")

	req := httptest.NewRequest("GET", "/artifacts/raw", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotAcceptable, w.Code)
}

func TestListRawArtifacts_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	setupArtifactsPath(t, dir)

	req := httptest.NewRequest("GET", "/artifacts/raw", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// nil slice encodes as JSON null
	var entries []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entries))
	assert.Empty(t, entries)
}

func TestListRawArtifacts_WithFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.bin", "c.log"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}
	setupArtifactsPath(t, dir)

	req := httptest.NewRequest("GET", "/artifacts/raw", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var entries []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entries))
	assert.ElementsMatch(t, []string{"a.txt", "b.bin", "c.log"}, entries)
}

func TestListRawArtifacts_DirNotExist(t *testing.T) {
	setupArtifactsPath(t, "/nonexistent/path/that/does/not/exist")

	req := httptest.NewRequest("GET", "/artifacts/raw", nil)
	w := httptest.NewRecorder()
	newRawRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
