package common

import "testing"

// testJobDriver is a JobDriver double used across the job tests in this
// package. Run streams the configured value or returns the configured error,
// and records the arguments it was called with.
type testJobDriver struct {
	id        JobDriverID
	config    any
	value     *string
	err       error
	buildErr  error
	lastArgs  []string
	callCount int
}

func (d *testJobDriver) ID() JobDriverID {
	return d.id
}

func (d *testJobDriver) Info() JobDriverInfo {
	return JobDriverInfo{
		ID: d.id,
		New: func(config any) (JobDriver, error) {
			if d.buildErr != nil {
				return nil, d.buildErr
			}
			return &testJobDriver{id: d.id, config: config, value: d.value, err: d.err}, nil
		},
	}
}

func (d *testJobDriver) Run(w JobResultWriter, args ...string) error {
	d.callCount++
	d.lastArgs = args
	if d.err != nil {
		return d.err
	}
	if d.value != nil {
		return w.WriteValue(*d.value)
	}
	return nil
}

func (d *testJobDriver) Config() JobDriverConfig {
	return JobDriverConfig{Driver: d.id, DriverConfig: d.config}
}

// registerTestJobDriver registers d in the global job driver registry and
// unregisters it when the test finishes.
func registerTestJobDriver(t *testing.T, d *testJobDriver) {
	t.Helper()

	if err := RegisterJobDriver(d); err != nil {
		t.Fatalf("failed to register test job driver: %v", err)
	}
	t.Cleanup(func() {
		_ = UnregisterJobDriver(d.ID())
	})
}
