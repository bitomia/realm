package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/clients/go"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

type UpdateClusterResult struct {
	Plan    []string `json:"plan"`
	Updated bool     `json:"updated"`
	Error   string   `json:"error,omitempty"`
}

func planAndUpdate(ctx context.Context, cfg *config.Config, stateFile string) UpdateClusterResult {
	result := UpdateClusterResult{Plan: []string{}}

	if _, err := clientPkg.PlanCluster(cfg, stateFile, func(diffs []string) (bool, error) {
		result.Plan = diffs
		return true, nil
	}); err != nil {
		result.Error = "plan failed: " + err.Error()
		return result
	}

	client, err := clientPkg.NewClient(cfg, stateFile)
	if err == nil {
		err = client.UpdateCluster(ctx)
	}
	if err != nil {
		result.Error = "update failed: " + err.Error()
		return result
	}
	result.Updated = true

	return result
}

// decodeNamed decodes a body whose name, if any, must match the path name
func decodeNamed[T any](r *http.Request, name string, bodyName func(*T) *string) (T, error) {
	var v T
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("invalid config: %w", err)
	}
	if n := bodyName(&v); *n != "" && *n != name {
		return v, fmt.Errorf("body name '%s' does not match path name '%s'", *n, name)
	}
	*bodyName(&v) = name
	return v, nil
}

func StartServer(cfg *config.Config, addr string, stateFile string) error {
	var mu sync.RWMutex

	// Must be called with mu locked
	applyConfig := func(r *http.Request, rebuild func() (*config.Config, error)) (UpdateClusterResult, error) {
		newCfg, err := rebuild()
		if err != nil {
			return UpdateClusterResult{}, err
		}
		*cfg = *newCfg

		return planAndUpdate(context.WithoutCancel(r.Context()), cfg, stateFile), nil
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()

		writeJSON(w, http.StatusOK, *cfg)
	})

	mux.HandleFunc("GET /config/nodes/{name}", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()

		node, ok := cfg.Nodes[r.PathValue("name")]
		if !ok {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, node)
	})

	mux.HandleFunc("POST /config/nodes/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		node, err := decodeNamed(r, name, func(n *common.NodeConfig) *string { return &n.Name })
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		_, existed := cfg.Nodes[name]
		result, err := applyConfig(r, func() (*config.Config, error) { return cfg.WithNode(name, node) })
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		status := http.StatusOK
		if !existed {
			status = http.StatusCreated
			w.Header().Set("Location", "/config/nodes/"+name)
		}

		writeJSON(w, status, map[string]any{"node": cfg.Nodes[name], "cluster": result})
	})

	mux.HandleFunc("DELETE /config/nodes/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		mu.Lock()
		defer mu.Unlock()

		if _, exists := cfg.Nodes[name]; !exists {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}

		result, err := applyConfig(r, func() (*config.Config, error) { return cfg.WithoutNode(name) })
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"cluster": result})
	})

	mux.HandleFunc("GET /config/loads/{name}", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()

		load, ok := cfg.Loads[r.PathValue("name")]
		if !ok {
			http.Error(w, "load not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, load)
	})

	mux.HandleFunc("POST /config/loads/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		load, err := decodeNamed(r, name, func(l *common.LoadConfig) *string { return &l.Name })
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		_, existed := cfg.Loads[name]
		result, err := applyConfig(r, func() (*config.Config, error) { return cfg.WithLoad(name, load) })
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		status := http.StatusOK
		if !existed {
			status = http.StatusCreated
			w.Header().Set("Location", "/config/loads/"+name)
		}

		writeJSON(w, status, map[string]any{"load": cfg.Loads[name], "cluster": result})
	})

	mux.HandleFunc("DELETE /config/loads/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		mu.Lock()
		defer mu.Unlock()

		if _, exists := cfg.Loads[name]; !exists {
			http.Error(w, "load not found", http.StatusNotFound)
			return
		}

		result, err := applyConfig(r, func() (*config.Config, error) { return cfg.WithoutLoad(name) })
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"cluster": result})
	})

	mux.HandleFunc("GET /config/jobs/{name}", func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		defer mu.RUnlock()

		job, ok := cfg.Jobs[r.PathValue("name")]
		if !ok {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, job)
	})

	mux.HandleFunc("POST /config/jobs/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		job, err := decodeNamed(r, name, func(j *common.JobConfig) *string { return &j.Name })
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		newCfg, err := cfg.WithJob(name, job)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		status := http.StatusOK
		if _, existed := cfg.Jobs[name]; !existed {
			status = http.StatusCreated
			w.Header().Set("Location", "/config/jobs/"+name)
		}
		*cfg = *newCfg

		writeJSON(w, status, cfg.Jobs[name])
	})

	mux.HandleFunc("DELETE /config/jobs/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		mu.Lock()
		defer mu.Unlock()

		if _, exists := cfg.Jobs[name]; !exists {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		newCfg, err := cfg.WithoutJob(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		*cfg = *newCfg

		w.WriteHeader(http.StatusNoContent)
	})

	slog.Info("Serving cluster", "address", addr, "state", stateFile)

	return http.ListenAndServe(addr, mux)
}

var serveCmd = &cobra.Command{
	Use:                   "serve",
	Short:                 "Serve the cluster",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		addr, _ := cmd.Flags().GetString("listen")
		if err := StartServer(cfg, addr, getStateFile(cmd)); err != nil {
			slog.Error("Server stopped", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().StringP("config", "c", "", "Path to configuration file (default: config.yaml)")
	serveCmd.Flags().StringP("listen", "l", ":8080", "Listen address")
	rootCmd.AddCommand(serveCmd)
}
