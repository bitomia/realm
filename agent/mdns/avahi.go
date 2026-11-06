package mdns

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	dbus "github.com/godbus/dbus/v5"
	avahi "github.com/holoplot/go-avahi"
)

const (
	avahiBusName = "org.freedesktop.Avahi"

	avahiEntryGroupEstablished = 2
	avahiEntryGroupCollision   = 3
	avahiEntryGroupFailure     = 4

	avahiRetryDelay = 5 * time.Second
)

type avahiPublisher struct {
	conn    *dbus.Conn
	server  *avahi.Server
	signals chan *dbus.Signal

	name        string
	serviceType string
	port        uint16
	txt         [][]byte
	ifaces      []int32

	stop chan struct{}
	done chan struct{}
}

func startAvahiPublisher(name, serviceType string, port int, txt []string, ifaces []net.Interface) (*avahiPublisher, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connecting to the system bus: %w", err)
	}

	var running bool
	if err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, avahiBusName).Store(&running); err != nil {
		conn.Close()
		return nil, fmt.Errorf("looking up avahi-daemon: %w", err)
	}
	if !running {
		conn.Close()
		return nil, errors.New("avahi-daemon is not running")
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, avahiBusName),
	); err != nil {
		conn.Close()
		return nil, fmt.Errorf("watching avahi-daemon: %w", err)
	}
	signals := make(chan *dbus.Signal, 10)
	conn.Signal(signals)

	server, err := avahi.ServerNew(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connecting to avahi-daemon: %w", err)
	}

	p := &avahiPublisher{
		conn:        conn,
		server:      server,
		signals:     signals,
		name:        name,
		serviceType: serviceType,
		port:        uint16(port),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	for _, t := range txt {
		p.txt = append(p.txt, []byte(t))
	}
	for _, iface := range ifaces {
		p.ifaces = append(p.ifaces, int32(iface.Index))
	}
	if len(p.ifaces) == 0 {
		p.ifaces = []int32{avahi.InterfaceUnspec}
	}

	go p.run()

	return p, nil
}

func (p *avahiPublisher) Stop() {
	close(p.stop)
	<-p.done
	p.server.Close()
	p.conn.Close()
}

func (p *avahiPublisher) run() {
	defer close(p.done)

	for {
		group, err := p.publish()
		if err != nil {
			slog.Warn("Publishing mDNS service with Avahi failed", "name", p.name, "error", err)
		}
		again := p.wait(group, err != nil)
		if group != nil {
			p.server.EntryGroupFree(group)
		}
		if !again {
			return
		}
	}
}

func (p *avahiPublisher) publish() (*avahi.EntryGroup, error) {
	group, err := p.server.EntryGroupNew()
	if err != nil {
		return nil, fmt.Errorf("creating entry group: %w", err)
	}

	for _, iface := range p.ifaces {
		err := group.AddService(iface, avahi.ProtoUnspec, 0, p.name, p.serviceType, "", "", p.port, p.txt)
		if err != nil {
			p.server.EntryGroupFree(group)
			return nil, fmt.Errorf("adding service on interface %d: %w", iface, err)
		}
	}

	if err := group.Commit(); err != nil {
		p.server.EntryGroupFree(group)
		return nil, fmt.Errorf("committing entry group: %w", err)
	}

	return group, nil
}

func (p *avahiPublisher) wait(group *avahi.EntryGroup, failed bool) bool {
	var states chan avahi.EntryGroupState
	if group != nil {
		states = group.StateChangeChannel
	}
	var retry <-chan time.Time
	if failed {
		retry = time.After(avahiRetryDelay)
	}

	for {
		select {
		case <-p.stop:
			return false

		case <-retry:
			return true

		case sig := <-p.signals:
			if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) != 3 {
				continue
			}
			if busName, _ := sig.Body[0].(string); busName != avahiBusName {
				continue
			}
			if newOwner, _ := sig.Body[2].(string); newOwner != "" {
				slog.Info("avahi-daemon (re)started, publishing mDNS service again", "name", p.name)
				return true
			}
			slog.Warn("avahi-daemon stopped, mDNS service unpublished until it is back", "name", p.name)
			states, retry = nil, nil

		case s, ok := <-states:
			if !ok {
				states = nil
				continue
			}
			switch s.State {
			case avahiEntryGroupEstablished:
				slog.Info("mDNS service published with Avahi", "name", p.name, "type", p.serviceType, "port", p.port)
			case avahiEntryGroupCollision:
				alt, err := p.server.GetAlternativeServiceName(p.name)
				if err != nil {
					slog.Warn("mDNS service name collision", "name", p.name, "error", err)
					retry = time.After(avahiRetryDelay)
					continue
				}
				slog.Warn("mDNS service name collision, renaming", "name", p.name, "new_name", alt)
				p.name = alt
				return true
			case avahiEntryGroupFailure:
				slog.Warn("Avahi failed to publish mDNS service", "name", p.name, "error", s.Error)
				retry = time.After(avahiRetryDelay)
			}
		}
	}
}
