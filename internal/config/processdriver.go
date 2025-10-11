package config

type ProcessDriver struct {
	StartCmd   string `json:"start_cmd"`
	StopSignal string `json:"stop_signal"`
}

func NewProcessDriverFromConfig(config ProcessConfig) *ProcessDriver {
	return &ProcessDriver{
		StartCmd: config.StartCmd,
		// TODO convert StopSignal to os.Signal
		StopSignal: config.StopSignal,
	}
}

func (p *ProcessDriver) GetDriverType() LoadDriverType {
	return ProcessDriverType
}
