package dns

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/miekg/dns"

	"github.com/bitomia/realm/daemon/db"
)

func forwardRequest(w dns.ResponseWriter, r *dns.Msg) {
	client := new(dns.Client)

	response, _, err := client.Exchange(r, "8.8.8.8:53")
	if err != nil {
		slog.Error("Failed to forward query to 8.8.8.8", "error", err)
		return
	}

	err = w.WriteMsg(response)
	if err != nil {
		slog.Error("Failed to send response to client", "error", err)
	}
}

func replyRealmDomainRequest(q *dns.Question, w dns.ResponseWriter, r *dns.Msg) {
	if q.Qtype == dns.TypeAAAA {
		// Skip ipv6 resolutions
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
		return
	}

	switch q.Qtype {
	case dns.TypeA:
		database := db.GetDB()

		containerName := q.Name[:len(q.Name)-len(".realm.")]
		key := containerNameToKey(containerName)
		value, err := database.GetDNSRecord(key)
		if err == nil && len(value) > 0 {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   q.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    0,
				},
				A: net.ParseIP(value),
			}
			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Answer = append(msg.Answer, rr)
			err = w.WriteMsg(msg)
			if err != nil {
				slog.Error("Failed to send response", "error", err)
			}
		} else {
			m := new(dns.Msg)
			m.SetReply(r)                // Copy the query header
			m.Rcode = dns.RcodeNameError // NXDOMAIN
			err := w.WriteMsg(m)
			if err != nil {
				slog.Error("Failed to write message", "error", err)
			}
		}
	}
}

func containerNameToKey(containerName string) string {
	return fmt.Sprintf("realm:dns:%s", containerName)
}

func HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	for _, q := range r.Question {
		if strings.HasSuffix(q.Name, ".realm.") {
			replyRealmDomainRequest(&q, w, r)
		} else {
			forwardRequest(w, r)
		}
	}
}

func RegisterContainerDNS(containerName string, ip net.IPNet) error {
	slog.Info("RegisterContainerDNS", "container", containerName, "ip", ip)

	database := db.GetDB()

	key := fmt.Sprintf("realm:dns:%s", containerName)
	return database.SetDNSRecord(key, ip.IP.String())
}

func UnregisterContainerDNS(containerName string) error {
	database := db.GetDB()

	key := containerNameToKey(containerName)
	return database.DeleteDNSRecord(key)
}

func Initialize() {
	dns.HandleFunc(".", HandleDNSRequest)

	server := &dns.Server{Addr: ":15353", Net: "udp"}
	go func() {
		slog.Info("Starting DNS server on :15353 (UDP)")
		if err := server.ListenAndServe(); err != nil {
			slog.Error("Failed to start DNS server", "error", err)
			os.Exit(1)
		}
	}()

	tcpServer := &dns.Server{Addr: ":15353", Net: "tcp"}
	go func() {
		slog.Info("Starting DNS server on port :15353 (TCP)")
		if err := tcpServer.ListenAndServe(); err != nil {
			slog.Error("Failed to start DNS server", "error", err)
			os.Exit(1)
		}
	}()

}
