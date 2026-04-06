package cloudinit

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"

	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/cloudinit"
)

var (
	mu    sync.RWMutex
	nodes = make(map[string]*cloudinit.CloudInit)
)

func RegisterNode(node *common.Node) error {
	if node == nil {
		return fmt.Errorf("node cannot be nil")
	}
	if node.CloudInit == nil {
		return fmt.Errorf("node %s has no cloud-init config", node.Name)
	}

	mu.Lock()
	defer mu.Unlock()
	nodes[node.Name] = node.CloudInit
	return nil
}

func UnregisterNode(nodeName string) error {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := nodes[nodeName]; !exists {
		return fmt.Errorf("node %s not registered", nodeName)
	}
	delete(nodes, nodeName)
	return nil
}

func RequestHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeName := vars["nodeName"]
	dataType := vars["dataType"]

	slog.Info("cloudinit.RequestHandler", "node", nodeName, "data", dataType)

	mu.RLock()
	defer mu.RUnlock()

	ci, exists := nodes[nodeName]

	if !exists {
		http.NotFound(w, r)
		return
	}

	var data any
	switch dataType {
	case "meta-data":
		if ci.MetaData == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		data = ci.MetaData
	case "user-data":
		if ci.UserData == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		data = ci.UserData
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		http.Error(w, "failed to marshal cloud-init data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/yaml")
	if dataType == "user-data" {
		w.Write([]byte("#cloud-config\n"))
	}
	w.Write(out)
}
