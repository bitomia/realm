package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/daemon/utils"
)

func DeleteProxyHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("[Caddy] DeleteProxyHandler", "container", containerName)

	err := proxy.DeleteReverseProxy(containerName)
	if err.Error != nil {
		utils.HttpError(w, http.StatusInternalServerError, "DeleteReverseProxy failed %s: %v", containerName, err)
		return
	}
}

func SetProxyHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	slog.Info("[Caddy] SetProxyHandler", "container", containerName)

	var opts proxy.ProxyOpts
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		utils.HttpError(w, http.StatusBadRequest, "%s", err.Error())
		return
	}

	caddyErr := proxy.SetReverseProxy(containerName, opts)
	if caddyErr.Error != nil {
		utils.HttpError(w, http.StatusInternalServerError, "SetReverseProxy failed %s: %v", containerName, caddyErr)
		return
	}
}

func GetProxyConfigHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	slog.Info("[Caddy] GetConfigHandler", "ID", id)

	statusCode, body, err := proxy.GetReverseProxyConfig(id)
	if err != nil {
		utils.HttpError(w, http.StatusInternalServerError, "GetConfigHandler failed %s: %v", id, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}
