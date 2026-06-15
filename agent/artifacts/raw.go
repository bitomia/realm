package artifacts

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
)

func GetRawArtifactHandler(w http.ResponseWriter, r *http.Request) {
	if rawArtifactsPath == nil {
		http.Error(w, "raw artifacts not available", http.StatusNotAcceptable)
		return
	}

	name := mux.Vars(r)["name"]
	target := filepath.Clean(filepath.Join(*rawArtifactsPath, name))

	slog.Info("artifacts.GetRawArtifactHandler", "target", target, "name", name)
	http.ServeFile(w, r, target)
}

func ListRawArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	if rawArtifactsPath == nil {
		http.Error(w, "raw artifacts not available", http.StatusNotAcceptable)
		return
	}

	slog.Info("artifacts.ListRawArtifactsHandler", "path", *rawArtifactsPath)
	dirEntries, err := os.ReadDir(*rawArtifactsPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var entries []string
	for _, entry := range dirEntries {
		entries = append(entries, entry.Name())
	}
	json.NewEncoder(w).Encode(entries)
}
