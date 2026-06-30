package agent

import (
	"github.com/gorilla/mux"

	"github.com/bitomia/realm/agent/auth"
	"github.com/bitomia/realm/agent/handlers"
)

func createBaseRoutes(router *mux.Router) {
	router.HandleFunc("/version", handlers.VersionHandler).Methods("GET")

	router.Handle("/system", auth.WithAuth(handlers.GetSystemInfoHandler)).Methods("GET")
	router.Handle("/images", auth.WithAuth(handlers.ListImagesHandler)).Methods("GET")
	router.Handle("/containers", auth.WithAuth(handlers.ListContainersHandler)).Methods("GET")
	router.Handle("/network", auth.WithAuth(handlers.ListNetworksHandler)).Methods("GET")

	router.Handle("/node", auth.WithAuth(handlers.NodeStateHandler)).Methods("GET")
	router.Handle("/node/config", auth.WithAuth(handlers.GetNodeConfigHandler)).Methods("GET")
	router.Handle("/node/config", auth.WithAuth(handlers.LoadNodeConfigHandler)).Methods("POST")
	router.Handle("/node/config", auth.WithAuth(handlers.UnloadNodeConfigHandler)).Methods("DELETE")
	router.Handle("/node/poweron", auth.WithAuth(handlers.PowerOnNodeHandler)).Methods("POST")
	router.Handle("/node/poweroff", auth.WithAuth(handlers.PowerOffNodeHandler)).Methods("POST")
	router.Handle("/node/shutdown", auth.WithAuth(handlers.ShutdownNodeHandler)).Methods("POST")
	router.Handle("/node/restart", auth.WithAuth(handlers.RestartNodeHandler)).Methods("POST")

	router.Handle("/loads", auth.WithAuth(handlers.GetLoadsDeploymentsHandler)).Methods("GET")
	router.Handle("/loads/provision", auth.WithAuth(handlers.ProvisionLoadHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/start", auth.WithAuth(handlers.StartLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/stop", auth.WithAuth(handlers.StopLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/deprovision", auth.WithAuth(handlers.DeprovisionLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/kill", auth.WithAuth(handlers.KillLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/stdout", auth.WithAuth(handlers.ReadLoadStdoutHandler)).Methods("GET")
	router.Handle("/loads/{loadName}/stderr", auth.WithAuth(handlers.ReadLoadStderrHandler)).Methods("GET")
}
