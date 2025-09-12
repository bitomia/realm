package proxy

import (
	"fmt"
	"log/slog"

	"github.com/bitomia/realm/internal/config"
)

func DeleteStaticProject(projectID string) error {
	caddyID := fmt.Sprintf("static_%s", projectID)
	masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
	masterCaddyURL = fmt.Sprintf("http://%s/id/%s", masterCaddyURL, caddyID)
	statusCode, body, err := HttpCaddyRequest(masterCaddyURL, "DELETE", nil)
	if err != nil {
		slog.Error("Error on DeleteStaticProject", "status", statusCode, "body", body, "error", err)
		return err
	}
	return nil
}

func AddStaticDomain(projectID string, domain string) error {
	masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
	masterCaddyURL = fmt.Sprintf("http://%s/id/static_%s/match/0/host", masterCaddyURL, projectID)

	caddyRequest := fmt.Sprintf(`"%s"`, domain)
	statusCode, body, err := HttpCaddyRequest(masterCaddyURL, "POST", &caddyRequest)
	if err != nil {
		slog.Error("Error on AddStaticDomain", "status", statusCode, "body", body, "error", err)
		return err
	}
	return nil
}

func RemoveStaticDomain(projectID string, domain string) error {
	caddyID := fmt.Sprintf("static_%s", projectID)

	slog.Info("RemoveStaticDomain", "step", 1, "projectID", projectID, "domain", domain, "caddyID", caddyID)

	masterHosts, err := GetMasterProxyHosts(caddyID)
	if err != nil {
		return err
	}
	masterHostsIndex := -1
	for index, host := range masterHosts {
		if host == domain {
			masterHostsIndex = index
			break
		}
	}

	slog.Info("RemoveStaticDomain", "step", 2, "projectID", projectID, "domain", domain, "caddyID", caddyID, "masterHostsIndex", masterHostsIndex)

	if masterHostsIndex != -1 {
		masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
		masterCaddyURL = fmt.Sprintf("http://%s/id/%s/match/0/host/%d", masterCaddyURL, caddyID, masterHostsIndex)
		HttpCaddyRequest(masterCaddyURL, "DELETE", nil)
	}

	return nil
}

func CreateStaticProject(projectID string, domain string) error {
	masterCaddyURL := config.Get().Daemon.MasterCaddyUrl
	masterCaddyURL = fmt.Sprintf("http://%s/config/apps/http/servers/master/routes", masterCaddyURL)

	caddyRequest := fmt.Sprintf(`
  {
    "@id": "static_%s",
    "handle": [
      {
        "handler": "cache",
        "ttl": "1h",
        "stale": "10m",
        "default_cache_control": "public, max-age=3600"
      },
      {
        "handler": "reverse_proxy",
        "upstreams": [
          {
            "dial": "sitecloud.website-us-east-1.linodeobjects.com:80"
          }
        ],
        "rewrite": {
          "uri": "/%s{http.request.uri}"
        },
        "headers": {
          "request": {
            "set": {
              "Host": ["sitecloud.website-us-east-1.linodeobjects.com"],
              "X-Real-IP": ["{http.request.remote.host}"]
            }
          }
        }
      }
    ],
    "match": [
      {
        "host": [
          "%s"
        ]
      }
    ]
  }
	`, projectID, projectID, domain)

	statusCode, body, err := HttpCaddyRequest(masterCaddyURL, "POST", &caddyRequest)
	if err != nil {
		slog.Error("Error on CreateStaticProject", "status", statusCode, "body", body, "error", err)
		return err
	}
	return nil
}
