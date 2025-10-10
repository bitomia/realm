package config

type ProcessDriver struct {
	startCmd   string
	stopSignal string
}

func NewProcessDriverFromConfig(config ProcessConfig) *ProcessDriver {
	return &ProcessDriver{
		startCmd: config.StartCmd,
		// TODO convert StopSignal to os.Signal
		stopSignal: config.StopSignal,
	}
}

func (p *ProcessDriver) GetDriverType() LoadDriverType {
	return ProcessDriverType
}
