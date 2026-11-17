package cmd

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/common/config"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}

func StartServer(cfg *config.Config, addr string) error {
	var mu sync.RWMutex

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

	slog.Info("Starting server", "address", addr)
	return http.ListenAndServe(addr, mux)
}

var serveCmd = &cobra.Command{
	Use:                   "serve",
	Short:                 "Serve the configuration over HTTP",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		addr, _ := cmd.Flags().GetString("listen")
		if err := StartServer(cfg, addr); err != nil {
			slog.Error("Server stopped", "error", err)
			os.Exit(1)
		}
	},
}

func init() {
	serveCmd.Flags().StringP("config", "c", "", "Path to configuration file, YAML or JSON (optional, default: config.yaml, config.yml or config.json in current working directory)")
	serveCmd.Flags().StringP("listen", "l", ":8080", "Address to listen on")
	rootCmd.AddCommand(serveCmd)
}
