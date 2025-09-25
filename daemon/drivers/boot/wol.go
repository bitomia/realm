package boot

type WakeOnLanDriver struct {
	host string
	port uint16
}

func NewWakeOnLanDriver(host string, port uint16) *WakeOnLanDriver {
	return &WakeOnLanDriver{host, port}
}

func (w *WakeOnLanDriver) Description() string {
	return "Boot driver for wake-on-lan"
}

func (w *WakeOnLanDriver) StartUp() {
	// TODO
}
