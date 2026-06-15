package artifacts

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/agent/auth"
	"github.com/bitomia/realm/common/config"
)

var (
	rawArtifactsPath *string
)

func Initialize(cfg *config.ArtifactsRepository, router *mux.Router) error {
	if cfg == nil {
		return nil
	}

	if router == nil {
		return fmt.Errorf("router cannot be nil on artifacts.Initialize")
	}

	if cfg.RawArtifactsPath != nil {
		absBase, err := filepath.Abs(*cfg.RawArtifactsPath)
		if err != nil {
			return fmt.Errorf("artifacts raw repository initialization failed: %w", err)
		}
		absBase += string(filepath.Separator)
		rawArtifactsPath = &absBase

		if cfg.AuthRequired {
			router.Handle("/artifacts/raw/{name}", auth.WithAuth(GetRawArtifactHandler)).Methods("GET")
			router.Handle("/artifacts/raw", auth.WithAuth(ListRawArtifactsHandler)).Methods("GET")
		} else {
			router.HandleFunc("/artifacts/raw/{name}", GetRawArtifactHandler).Methods("GET")
			router.HandleFunc("/artifacts/raw", ListRawArtifactsHandler).Methods("GET")
		}

		slog.Info("Artifacts raw repository enabled", "path", absBase, "auth_required", cfg.AuthRequired)
	}

	return nil
}
