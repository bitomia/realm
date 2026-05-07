package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/bitomia/realm/agent/cruntime"
	"github.com/bitomia/realm/agent/db"
	"github.com/bitomia/realm/agent/network"
)

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

	dbConn := db.GetDB()
	vethBridges, err := network.GetBridgeVethLinks()
	if err != nil {
		log.Printf("Failed to get bridge veth links: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	networks := make(map[string]*NetworkInfo)

	for _, container := range containersList {
		configs, _ := dbConn.GetNetConfigs(container.ID)

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
			_ = json.Unmarshal([]byte(config.CniResult), &result)
			networks[container.ID].Addresses = append(networks[container.ID].Addresses, result.Addresses...)
		}
	}
	_ = json.NewEncoder(w).Encode(networks)
}
