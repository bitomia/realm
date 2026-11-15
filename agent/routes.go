package agent

import (
	"github.com/gorilla/mux"

	"github.com/bitomia/realm/agent/auth"
	"github.com/bitomia/realm/agent/handlers"
)

func createBaseRoutes(router *mux.Router) {
	router.HandleFunc("/version", handlers.VersionHandler).Methods("GET")

	router.Handle("/system", auth.WithAuth(handlers.GetSystemInfo)).Methods("GET")

	router.Handle("/node", auth.WithAuth(handlers.GetNodeState)).Methods("GET")
	router.Handle("/node/config", auth.WithAuth(handlers.GetNodeConfig)).Methods("GET")
	router.Handle("/node/config", auth.WithAuth(handlers.LoadNodeConfig)).Methods("POST")
	router.Handle("/node/config", auth.WithAuth(handlers.UnloadNodeConfig)).Methods("DELETE")
	router.Handle("/node/poweron", auth.WithAuth(handlers.PowerOnNode)).Methods("POST")
	router.Handle("/node/poweroff", auth.WithAuth(handlers.PowerOffNode)).Methods("POST")
	router.Handle("/node/shutdown", auth.WithAuth(handlers.ShutdownNode)).Methods("POST")
	router.Handle("/node/restart", auth.WithAuth(handlers.RestartNode)).Methods("POST")

	router.Handle("/loads", auth.WithAuth(handlers.GetLoadsDeploymentsHandler)).Methods("GET")
	router.Handle("/loads/{loadName}/start", auth.WithAuth(handlers.StartLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/stop", auth.WithAuth(handlers.StopLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/kill", auth.WithAuth(handlers.KillLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/stdout", auth.WithAuth(handlers.ReadLoadStdoutHandler)).Methods("GET")
	router.Handle("/loads/{loadName}/stderr", auth.WithAuth(handlers.ReadLoadStderrHandler)).Methods("GET")

	router.Handle("/jobs", auth.WithAuth(handlers.RunJobHandler)).Methods("POST")
}
