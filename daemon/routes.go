package daemon

import (
	"github.com/gorilla/mux"

	"github.com/bitomia/realm/daemon/auth"
	"github.com/bitomia/realm/daemon/handlers"
)

func createRoutes(router *mux.Router) {
	router.HandleFunc("/version", handlers.VersionHandler).Methods("GET")
	router.Handle("/state", auth.WithAuth(handlers.GetNodeStateHandler)).Methods("GET")
	router.Handle("/system", auth.WithAuth(handlers.GetSystemInfoHandler)).Methods("GET")
	router.Handle("/images", auth.WithAuth(handlers.ListImagesHandler)).Methods("GET")
	router.Handle("/containers", auth.WithAuth(handlers.ListContainersHandler)).Methods("GET")
	router.Handle("/network", auth.WithAuth(handlers.ListNetworksHandler)).Methods("GET")

	router.Handle("/containers/{container}/logs", auth.WithAuth(handlers.ReadContainerLogsHandler)).Methods("GET")

	router.Handle("/node/plan", auth.WithAuth(handlers.PlanAndRegisterNodeHandler)).Methods("POST")
	router.Handle("/node/shutdown", auth.WithAuth(handlers.ShutdownNodeHandler)).Methods("POST")
	router.Handle("/node/restart", auth.WithAuth(handlers.RestartNodeHandler)).Methods("POST")

	router.Handle("/loads", auth.WithAuth(handlers.GetLoadsDeploymentsHandler)).Methods("GET")
	router.Handle("/loads/plan", auth.WithAuth(handlers.PlanAndRegisterLoadHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/start", auth.WithAuth(handlers.StartLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/stop", auth.WithAuth(handlers.StopLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/unplan", auth.WithAuth(handlers.UnplanLoadHandler)).Methods("POST")
}
