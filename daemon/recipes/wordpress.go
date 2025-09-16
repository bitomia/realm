package recipes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/bitomia/realm/daemon/containers"
	"github.com/bitomia/realm/daemon/network"
	"github.com/bitomia/realm/daemon/proxy"
	"github.com/bitomia/realm/daemon/utils"
	"github.com/bitomia/realm/internal/requests"
)

type WordpressRecipeOpts struct {
	Title      string   `json:"title"`
	Locale     string   `json:"locale"`
	AdminUser  string   `json:"admin_user"`
	AdminEmail string   `json:"admin_email"`
	Hosts      []string `json:"hosts"`
}

func (o WordpressRecipeOpts) Validate() bool {
	if len(o.Title) == 0 {
		return false
	}
	if len(o.Locale) == 0 {
		return false
	}
	if len(o.AdminUser) == 0 {
		return false
	}
	if len(o.AdminEmail) == 0 {
		return false
	}
	if len(o.Hosts) == 0 {
		return false
	}
	return true
}

type WordpressPlanOpts struct {
	WPVolumeSize string `json:"wpVolumeSize"`
	WPMemLimit   uint64 `json:"wpMemLimit"`
	DBVolumeSize string `json:"dbVolumeSize"`
	DBMemLimit   uint64 `json:"dbMemLimit"`
}

func appendArgsToEnv(args map[string]string, env []string) []string {
	for a, b := range args {
		env = append(env, fmt.Sprintf("%s=%s", a, b))
	}
	return env
}

func checkEndpointReadiness(url string, timeout time.Duration) error {
	client := http.Client{Timeout: timeout}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
}

func WaitEndpointReady(url string, progressMessage string, baseProgress float32, maxProgress float32, w http.ResponseWriter, flusher http.Flusher) bool {
	timeout := 10 * time.Second // Timeout for each HTTP
	interval := 5 * time.Second // Interval between retries
	maxRetries := 30            // Maximum number of retries

	progressInc := (maxProgress - baseProgress) / float32(maxRetries)
	for i := 0; i < maxRetries; i++ {
		slog.Info("Checking readiness", "url", url, "retry", i+1, "maxRetries", maxRetries)
		err := checkEndpointReadiness(url, timeout)
		if err == nil {
			return true
		}
		slog.Info("Endpoint still not ready", "url", url, "error", err)
		time.Sleep(interval)

		json.NewEncoder(w).Encode(Progress{baseProgress + progressInc*float32(i), progressMessage})
		flusher.Flush()
	}
	return false
}

func WaitForTCPPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err == nil {
			// Connection successful, close it and return nil
			conn.Close()
			return nil
		}
		// Wait for a short interval before retrying
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout reached while waiting for %s", address)
}

type Progress struct {
	Progress float32 `json:"progress"`
	Message  string  `json:"message"`
}

type RecipeWordpressRet struct {
	RecipeID  uuid.UUID
	SSOSecret string
}

func LaunchWordpress(w http.ResponseWriter, recipeId uuid.UUID, recipeOpts WordpressRecipeOpts, planOpts WordpressPlanOpts) (*RecipeWordpressRet, error) {
	slog.Info("LaunchWordpress", "recipeID", recipeId.String(), "opts", planOpts)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return nil, errors.ErrUnsupported
	}

	// Start mariadb
	mariadbContainerName := fmt.Sprintf("%s_%s", "mariadb", recipeId)
	env := []string{
		"ALLOW_EMPTY_PASSWORD=no",
		"MARIADB_USER=wordpress",
		"MARIADB_PASSWORD=wordpress",
		"MARIADB_DATABASE=wordpress",
		"MARIADB_ROOT_PASSWORD=pwd",
	}

	dbVolumeSize := planOpts.DBVolumeSize
	//dbMemLimit := planOpts.dbMemLimit
	createContainerOpts := requests.CreateContainerOpts{
		Image:            "docker.io/bitnami/mariadb:latest",
		VolumeMountPoint: "/bitnami/mariadb",
		Quotas: requests.Quotas{
			VolumeSize: &dbVolumeSize,
			//			MemLimit:   &dbMemLimit,
		},
		Env: env,
	}
	slog.Info("LaunchWordpress - Start mariadb", "recipeID", recipeId.String(), "env", env)

	json.NewEncoder(w).Encode(Progress{0.2, "starting_database"})
	flusher.Flush()

	if err := containers.CreateContainer(mariadbContainerName, createContainerOpts, nil); err != nil {
		return nil, err
	}

	updateStateOpts := containers.UpdateContainerOpts{
		State: "start",
	}
	if err := containers.UpdateContainerState(mariadbContainerName, updateStateOpts); err != nil {
		slog.Info("UpdateContainerState failed on wordpress_starter recipe", "error", err.Error())
		opts := containers.DeleteContainerOpts{
			RemoveVolume: true,
		}
		if containers.DeleteContainer(mariadbContainerName, opts, syscall.SIGKILL, true, true) != nil {
			slog.Info("DeleteContainer for UpdateContainerState failed on LaunchWordpress", "container", mariadbContainerName)
		}
		return nil, err
	}
	startNetworkOpts := network.StartNetworkOpts{
		Network: recipeId.String(),
		IPMasq:  false,
		DNS:     true,
	}
	err, _, _, _ := network.StartNetwork(mariadbContainerName, startNetworkOpts)
	if err != nil {
		slog.Info("network.StartNetwork on recipe %s failed", "container", mariadbContainerName, "error", err.Error())
		opts := containers.DeleteContainerOpts{
			RemoveVolume: true,
		}
		if containers.DeleteContainer(mariadbContainerName, opts, syscall.SIGKILL, true, true) != nil {
			slog.Info("DeleteContainer for StartNetwork failed on LaunchWordpress", "container", mariadbContainerName)
		}
		return nil, err
	}
	slog.Info("LaunchWordpress - mariadb started", "recipeID", recipeId.String())

	ssoSecretKey, err := utils.GenerateKey(16)
	if err != nil {
		slog.Error("Cannot generate sso secret key", "recipeID", recipeId, "error", err)
		return nil, err
	}

	// Start wordpress
	{
		json.NewEncoder(w).Encode(Progress{0.4, "starting_wordpress"})
		flusher.Flush()

		wpContainerName := fmt.Sprintf("%s_%s", "wp", recipeId)
		randomPassword, _ := utils.GenerateKey(10)

		env := []string{
			fmt.Sprintf("SCSSO_KEY=%s", ssoSecretKey),
			"ALLOW_EMPTY_PASSWORD=no",
			fmt.Sprintf("WORDPRESS_DATABASE_HOST=%s", mariadbContainerName+".realm"),
			"WORDPRESS_DATABASE_PORT_NUMBER=3306",
			"WORDPRESS_DATABASE_USER=wordpress",
			"WORDPRESS_DATABASE_PASSWORD=wordpress",
			"WORDPRESS_DATABASE_NAME=wordpress",
			"WORDPRESS_ENABLE_REVERSE_PROXY=yes",
			"WORDPRESS_ENABLE_HTTPS=yes",
			"WORDPRESS_PLUGINS=none",

			fmt.Sprintf("WORDPRESS_BLOG_NAME=%s", recipeOpts.Title),
			fmt.Sprintf("WORDPRESS_EMAIL=%s", recipeOpts.AdminEmail),
			fmt.Sprintf("WORDPRESS_EXTRA_INSTALL_ARGS=--url=%s --locale=%s --admin_user=%s --admin_password=%s", recipeOpts.Hosts[0], recipeOpts.Locale, recipeOpts.AdminUser, randomPassword),

			fmt.Sprintf("WORDPRESS_EXTRA_WP_CONFIG_CONTENT=define('SCSSO_KEY', '%s'); define('WP_MEMORY_LIMIT', '256M'); define('WP_MAX_MEMORY_LIMIT', '512M'); ini_set('max_execution_time', 300);", ssoSecretKey),
		}

		slog.Info("LaunchWordpress - Start wordpress", "recipeID", recipeId.String(), "env", env)
		wpVolumeSize := planOpts.WPVolumeSize
		wpMemLimit := planOpts.WPMemLimit

		createContainerOpts := requests.CreateContainerOpts{
			Image:            "ghcr.io/bitomia/wordpress-nginx:6.7.1",
			VolumeMountPoint: "/bitnami/wordpress",
			Quotas: requests.Quotas{
				VolumeSize: &wpVolumeSize,
				MemLimit:   &wpMemLimit,
			},
			Env: env,
		}
		if err := containers.CreateContainer(wpContainerName, createContainerOpts, nil); err != nil {
			return nil, err
		}

		updateStateOpts := containers.UpdateContainerOpts{
			State: "start",
		}
		if err := containers.UpdateContainerState(wpContainerName, updateStateOpts); err != nil {
			opts := containers.DeleteContainerOpts{
				RemoveVolume: true,
			}
			if containers.DeleteContainer(wpContainerName, opts, syscall.SIGKILL, true, true) != nil {
				slog.Info("DeleteContainer for UpdateContainerState failed on LaunchWordpress", "container", wpContainerName)
			}
			return nil, err
		}

		startNetworkOpts := network.StartNetworkOpts{
			Network: recipeId.String(),
			IPMasq:  true,
			DNS:     true,
		}
		err, _, _, wpIP := network.StartNetwork(wpContainerName, startNetworkOpts)
		if err != nil {
			opts := containers.DeleteContainerOpts{
				RemoveVolume: true,
			}
			if containers.DeleteContainer(wpContainerName, opts, syscall.SIGKILL, true, true) != nil {
				slog.Info("DeleteContainer for StartNetwork failed on LaunchWordpress", "container", wpContainerName)
			}
			return nil, err
		}
		slog.Info("LaunchWordpress - Wordpress started", "recipeID", recipeId.String())

		// TODO wait for mariadb with tcp connection
		address := fmt.Sprintf("%s.realm:3306", mariadbContainerName)
		timeout := 20 * time.Second
		err = WaitForTCPPort(address, timeout)
		if err != nil {
			slog.Error("Wordpress failed due to mariadb timeout", "container", mariadbContainerName)
			return nil, err
		}
		slog.Info("Wordpress mariadb ready", "container", mariadbContainerName)

		// Install wordpress
		slog.Info("LaunchWordpress - Install wordpress", "recipeID", recipeId.String())

		// Wait for wordpress ready
		WaitEndpointReady(fmt.Sprintf("http://%s:8080", wpIP), "installing_wordpress", 0.6, 0.9, w, flusher)

		json.NewEncoder(w).Encode(Progress{0.6, "installing_wordpress"})
		flusher.Flush()

		// Launch installation task
		slog.Info("LaunchWordpress - Wordpress installed", "recipeID", recipeId.String())
		json.NewEncoder(w).Encode(Progress{0.9, "wordpress_ready"})
		flusher.Flush()

		// Create reverse proxy
		slog.Info("LaunchWordpress - Creating reverse proxy", "recipeID", recipeId.String())
		proxyOpts := proxy.ProxyOpts{
			Hosts:         recipeOpts.Hosts,
			Upstream:      fmt.Sprintf(`%s.realm:8443`, wpContainerName),
			HttpsUpstream: true,
		}
		caddyError := proxy.SetReverseProxy(wpContainerName, proxyOpts)
		if caddyError.Error != nil {
			slog.Error("SetReverseProxy failed", "container", wpContainerName, "opts", proxyOpts, "body", caddyError.Body)
			return nil, caddyError.Error
		}
		slog.Info("LaunchWordpress - Reverse proxy created", "recipeID", recipeId.String())
	}
	json.NewEncoder(w).Encode(Progress{1.0, "done"})
	flusher.Flush()

	return &RecipeWordpressRet{recipeId, ssoSecretKey}, nil
}

// TODO NOTE: this is not deleting volumes
func RollbackWordpress(recipeId string) error {
	slog.Info("RollbackWordpress", "recipeID", recipeId)

	mariadbContainerName := fmt.Sprintf("%s_%s", "mariadb", recipeId)
	opts := containers.DeleteContainerOpts{
		RemoveVolume: true,
	}
	err := containers.DeleteContainer(mariadbContainerName, opts, syscall.SIGKILL, true, true)
	if err != nil {
		if err.Code == 1 {
			slog.Error("Ignored error while deleting container because it doesn't exist", "container", mariadbContainerName)
		} else {
			return err
		}
	}

	wpContainerName := fmt.Sprintf("%s_%s", "wp", recipeId)
	opts = containers.DeleteContainerOpts{}
	err = containers.DeleteContainer(wpContainerName, opts, syscall.SIGKILL, true, true)
	if err != nil {
		if err.Code == 1 {
			slog.Error("Ignored error while deleting container because it doesn't exist", "container", wpContainerName)
		} else {
			return err
		}
	}

	caddyError := proxy.DeleteReverseProxy(wpContainerName)
	if caddyError.Error != nil {
		slog.Error("Ignored error while deleting reverse proxy", "container", wpContainerName, "error", caddyError)
	}

	return nil
}

func AddWordpressDomain(recipeId string, domain string) error {
	wpContainerName := fmt.Sprintf("%s_%s", "wp", recipeId)
	err := proxy.AddProxyHost(wpContainerName, strings.ToLower(domain))
	if err != nil {
		return err
	} else {
		return nil
	}
}

func RemoveWordpressDomain(recipeId string, domain string) error {
	wpContainerName := fmt.Sprintf("%s_%s", "wp", recipeId)
	err := proxy.RemoveProxyHost(wpContainerName, strings.ToLower(domain))
	if err != nil {
		return err
	} else {
		return nil
	}
}
