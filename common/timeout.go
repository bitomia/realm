package common

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultStartTimeout = 5 * time.Minute
	DefaultStopTimeout  = 30 * time.Second
)

type TimeoutConfig struct {
	Start Duration `json:"start,omitempty"`
	Stop  Duration `json:"stop,omitempty"`
}

func (t TimeoutConfig) WithDefaults() TimeoutConfig {
	if t.Start <= 0 {
		t.Start = Duration(DefaultStartTimeout)
	}
	if t.Stop <= 0 {
		t.Stop = Duration(DefaultStopTimeout)
	}
	return t
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("invalid duration %s: %w", data, err)
	}
	return d.UnmarshalText([]byte(s))
}

func (d *Duration) UnmarshalText(text []byte) error {
	s := string(text)
	parsed, err := time.ParseDuration(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}

	*d = Duration(parsed)
	return nil
}
