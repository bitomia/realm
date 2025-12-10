package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/gorilla/mux"

	"github.com/bitomia/realm/daemon/cruntime"
	"github.com/bitomia/realm/daemon/db"
	"github.com/bitomia/realm/daemon/network"

	"github.com/bitomia/realm/common/dto"
)

func LinkContainerToNetworkHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	var opts dto.StartNetworkRequest
	err := json.NewDecoder(r.Body).Decode(&opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err, config, _, _ := network.StartNetwork(containerName, opts)
	if err != nil {
		log.Print(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(config)
}

func UnlinkContainerFromNetworkHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]

	if err := network.DeleteNetwork(containerName); err != nil {
		log.Printf("%s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
}

type NetworkInfo struct {
	Addresses      []network.IP `json:"addresses"`
	GuestIfaceName string       `json:"guest_iface"`
	HostIfaceName  string       `json:"host_iface"`
	VethLinks      []string     `json:"veth_links"`
}

func ListNetworksHandler(w http.ResponseWriter, _ *http.Request) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	containersList, err := client.ContainerService().List(ctx)
	if err != nil {
		log.Printf("Failed to list containers: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db := db.GetDB()
	vethBridges := network.GetBridgeVethLinks()
	networks := make(map[string]*NetworkInfo)

	for _, container := range containersList {
		configs, _ := db.GetNetConfigs(container.ID)

		if _, ok := networks[container.ID]; !ok {
			networks[container.ID] = &NetworkInfo{}
		}
		for _, config := range configs {
			networks[container.ID].GuestIfaceName = config.GuestIfaceName
			networks[container.ID].HostIfaceName = config.HostIfaceName

			if bridge, ok := vethBridges[config.HostIfaceName]; ok {
				for _, link := range bridge.VethLinks {
					networks[container.ID].VethLinks = append(networks[container.ID].VethLinks, link.Attrs().Name)
				}
			} else {
				log.Println("Unexpected condition bridge network not found")
			}
			var result network.IPAddresses
			json.Unmarshal([]byte(config.CniResult), &result)
			for _, ip := range result.Addresses {
				networks[container.ID].Addresses = append(networks[container.ID].Addresses, ip)
			}
		}
	}
	json.NewEncoder(w).Encode(networks)
}

func RepairNetworkHandler(w http.ResponseWriter, r *http.Request) {
	containerName := mux.Vars(r)["container"]
	log.Printf("RepairNetworkHandler %s", containerName)

	error := network.RepairNetwork(containerName)
	if error != nil {
		http.Error(w, error.Error(), http.StatusInternalServerError)
		return
	}
}

type PurgedNetworkInfo struct {
	CNIPaths []string `json:"cni_paths"`
	Bridges  []string `json:"bridges"`
}

func PurgeNetworksHandler(w http.ResponseWriter, r *http.Request) {
	ctx, client, err := cruntime.CreateClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer client.Close()

	containersList, err := client.ContainerService().List(ctx)
	if err != nil {
		log.Printf("Failed to list containers: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db := db.GetDB()
	hostIfaces := []string{}
	for _, container := range containersList {
		configs, _ := db.GetNetConfigs(container.ID)
		for _, config := range configs {
			hostIfaces = append(hostIfaces, config.HostIfaceName)
		}
	}

	result := PurgedNetworkInfo{}

	// Purge orphaned bridgets and links
	bridges := network.GetBridgeVethLinks()
	for bridgeName, bridge := range bridges {
		if !slices.Contains(hostIfaces, bridgeName) {
			log.Printf("Purging %s orphaned bridge network", bridgeName)
			if err := network.PurgeBridgeNetwork(bridge); err != nil {
				log.Printf("Ignoring puring bridge error: %s", err.Error())
			} else {
				result.Bridges = append(result.Bridges, bridge.Link.Attrs().Name)
			}
		}
	}

	// Purge orphaned CNI networks
	networkDir := "/var/lib/cni/networks"
	files, err := os.ReadDir(networkDir)
	if err != nil {
		log.Printf("Failed to read CNI networks dir: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, file := range files {
		if !slices.Contains(hostIfaces, file.Name()) {
			path := filepath.Join(networkDir, file.Name())
			log.Printf("Purging %s orphaned CNI config: %s", file.Name(), path)
			os.RemoveAll(path)
			result.CNIPaths = append(result.CNIPaths, path)
		}
	}

	json.NewEncoder(w).Encode(result)
}
