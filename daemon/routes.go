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
	router.Handle("/images", auth.WithAuth(handlers.PullImageHandler)).Methods("POST")

	router.Handle("/containers", auth.WithAuth(handlers.ListContainersHandler)).Methods("GET")
	router.Handle("/containers/{container}", auth.WithAuth(handlers.CreateContainerHandler)).Methods("POST")
	router.Handle("/containers/{container}/state", auth.WithAuth(handlers.UpdateContainerStateHandler)).Methods("PUT")
	router.Handle("/containers/{container}/quotas", auth.WithAuth(handlers.UpdateContainerQuotasHandler)).Methods("PUT")
	router.Handle("/containers/{container}", auth.WithAuth(handlers.RemoveContainerHandler)).Methods("DELETE")
	router.Handle("/containers/{container}/repair", auth.WithAuth(handlers.RepairContainerHandler)).Methods("POST")
	router.Handle("/containers/{container}/signal", auth.WithAuth(handlers.SendContainerSignalHandler)).Methods("POST")
	router.Handle("/containers/{container}/migrate", auth.WithAuth(handlers.MigrateContainerHandler)).Methods("POST")
	router.Handle("/containers/{container}/logs", auth.WithAuth(handlers.ReadContainerLogsHandler)).Methods("GET")
	router.Handle("/containers/{container}/network", auth.WithAuth(handlers.LinkContainerToNetworkHandler)).Methods("POST")
	router.Handle("/containers/{container}/network", auth.WithAuth(handlers.UnlinkContainerFromNetworkHandler)).Methods("DELETE")
	router.Handle("/containers/{id}/server", auth.WithAuth(handlers.GetProxyConfigHandler)).Methods("GET")
	router.Handle("/containers/{container}/proxy", auth.WithAuth(handlers.SetProxyHandler)).Methods("POST")
	router.Handle("/containers/{container}/proxy", auth.WithAuth(handlers.DeleteProxyHandler)).Methods("DELETE")

	router.Handle("/network", auth.WithAuth(handlers.ListNetworksHandler)).Methods("GET")
	router.Handle("/network", auth.WithAuth(handlers.PurgeNetworksHandler)).Methods("POST")
	router.Handle("/network/{container}/repair", auth.WithAuth(handlers.RepairNetworkHandler)).Methods("POST")

	router.Handle("/loads", auth.WithAuth(handlers.GetLoadStatesHandler)).Methods("GET")
	router.Handle("/loads/plan", auth.WithAuth(handlers.PlanLoadHandler)).Methods("POST")
	router.Handle("/loads/start", auth.WithAuth(handlers.StartLoadHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/stop", auth.WithAuth(handlers.StopLoadDeploymentsHandler)).Methods("POST")
	router.Handle("/loads/{loadName}/unplan", auth.WithAuth(handlers.UnplanLoadHandler)).Methods("POST")
}
