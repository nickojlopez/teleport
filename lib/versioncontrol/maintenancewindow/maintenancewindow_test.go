/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package maintenancewindow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/stretchr/testify/require"
)

type fakeKubeBackend struct {
	data map[string]string
}

func newFakeKubeBackend() *fakeKubeBackend {
	return &fakeKubeBackend{
		data: make(map[string]string),
	}
}

func (b *fakeKubeBackend) Put(ctx context.Context, item backend.Item) (*backend.Lease, error) {
	b.data[string(item.Key)] = string(item.Value)
	return nil, nil
}

func TestKubeControllerDriver(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bk := newFakeKubeBackend()

	driver, err := NewKubeControllerDriver(KubeControllerDriverConfig{
		Backend: bk,
	})
	require.NoError(t, err)

	require.Equal(t, "kube", driver.Kind())

	// verify basic schedule creation
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{
		KubeControllerSchedule: "fake-schedule",
	})
	require.NoError(t, err)

	key := "agent-maintenance-schedule"

	require.Equal(t, "fake-schedule", bk.data[key])

	// verify overwrite of existing schedule
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{
		KubeControllerSchedule: "fake-schedule-2",
	})
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-2", bk.data[key])

	// verify reset of schedule
	err = driver.Reset(ctx)
	require.NoError(t, err)

	require.Equal(t, "", bk.data[key])

	// verify reset of empty schedule has no effect
	err = driver.Reset(ctx)
	require.NoError(t, err)

	require.Equal(t, "", bk.data[key])

	// setup another fake schedule
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{
		KubeControllerSchedule: "fake-schedule-3",
	})
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-3", bk.data[key])

	// verify that empty schedule is equivalent to reset
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{})
	require.NoError(t, err)

	require.Equal(t, "", bk.data[key])
}

// TestSystemdUnitDriver verifies the basic behavior of the systemd unit export driver.
func TestSystemdUnitDriver(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// use a sub-directory of a temp dir in order to verify that
	// driver creates dir when needed.
	dir := filepath.Join(t.TempDir(), "config")

	driver, err := NewSystemdUnitDriver(SystemdUnitDriverConfig{
		ConfigDir: dir,
	})
	require.NoError(t, err)

	require.Equal(t, driver.Kind(), "unit")

	// verify basic schedule creation
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{
		SystemdUnitSchedule: "fake-schedule",
	})
	require.NoError(t, err)

	schedPath := filepath.Join(dir, "schedule")

	sb, err := os.ReadFile(schedPath)
	require.NoError(t, err)

	require.Equal(t, "fake-schedule", string(sb))

	// verify overwrite of existing schedule
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{
		SystemdUnitSchedule: "fake-schedule-2",
	})
	require.NoError(t, err)

	sb, err = os.ReadFile(schedPath)
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-2", string(sb))

	// verify reset/deletion of schedule
	err = driver.Reset(ctx)
	require.NoError(t, err)

	_, err = os.ReadFile(schedPath)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

	// verify that NotExist error is suppressed
	err = driver.Reset(ctx)
	require.NoError(t, err)

	// set up another schedule
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{
		SystemdUnitSchedule: "fake-schedule-3",
	})
	require.NoError(t, err)

	sb, err = os.ReadFile(schedPath)
	require.NoError(t, err)

	require.Equal(t, "fake-schedule-3", string(sb))

	// verify that an empty schedule value is treated equivalent to a reset
	err = driver.Sync(ctx, proto.ExportMaintenanceWindowsResponse{})
	require.NoError(t, err)

	_, err = os.ReadFile(schedPath)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}

// fakeDriver is used to inject custom behavior into a dummy Driver instance.
type fakeDriver struct {
	mu    sync.Mutex
	kind  string
	sync  func(context.Context, proto.ExportMaintenanceWindowsResponse) error
	reset func(context.Context) error
}

func (d *fakeDriver) Kind() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.kind != "" {
		return d.kind
	}
	return "fake"
}

func (d *fakeDriver) Sync(ctx context.Context, rsp proto.ExportMaintenanceWindowsResponse) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.sync != nil {
		return d.sync(ctx, rsp)
	}

	return nil
}

func (d *fakeDriver) Reset(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.reset != nil {
		return d.reset(ctx)
	}

	return nil
}

func (d *fakeDriver) withLock(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	fn()
}

func TestExporterBasics(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := make(chan context.Context)

	testEvents := make(chan testEvent, 1024)

	// set up fake export func that can be set to fail multiple times in sequence
	var exportCount int
	var exportFail bool
	var exportFlaky bool
	var exportLock sync.Mutex
	export := func(ctx context.Context, req proto.ExportMaintenanceWindowsRequest) (rsp proto.ExportMaintenanceWindowsResponse, err error) {
		if req.UpgraderKind != "fake" {
			panic("unexpected upgrader kind") // sanity check, shouldn't ever happen in practice
		}
		rsp.SystemdUnitSchedule = "fake-schedule"
		exportLock.Lock()
		exportCount++
		if exportFlaky && exportCount%2 == 0 {
			err = fmt.Errorf("fake-export-flaky")
		}
		if exportFail {
			err = fmt.Errorf("fake-export-fail")
		}
		exportLock.Unlock()
		return
	}

	driver := new(fakeDriver)

	driver.withLock(func() {
		driver.sync = func(ctx context.Context, rsp proto.ExportMaintenanceWindowsResponse) error {
			if rsp.SystemdUnitSchedule != "fake-schedule" {
				panic("unexpected schedule value") // sanity check, shouldn't ever happen in practice
			}
			return nil
		}
	})

	exporter, err := NewExporter(ExporterConfig[context.Context]{
		Driver:                   driver,
		ExportFunc:               export,
		AuthConnectivitySentinel: sc,
		UnhealthyThreshold:       time.Millisecond * 200,
		ExportInterval:           time.Millisecond * 300,
		FirstExport:              time.Millisecond * 10,
		testEvents:               testEvents,
	})
	require.NoError(t, err)

	go exporter.Run()
	defer exporter.Close()

	// without connection sentinel, exporter is unable to make progress. eventually forces reset.
	awaitEvents(t, testEvents,
		expect(resetFromRun),
		deny(sentinelAcquired, exportAttempt),
	)

	s1, s1Cancel := context.WithCancel(ctx)

	// provide a connection sentinel
	sc <- s1

	// wait until sentinel is acquired
	awaitEvents(t, testEvents,
		expect(sentinelAcquired),
	)

	// everything should now appear healthy/normal for multiple export cycles
	awaitEvents(t, testEvents,
		expect(exportAttempt, exportSuccess, exportSuccess),
		deny(resetFromRun, resetFromExport, getExportErr, syncExportErr, sentinelLost),
	)

	// introduce intermittent sync failures
	driver.withLock(func() {
		var si int
		driver.sync = func(ctx context.Context, rsp proto.ExportMaintenanceWindowsResponse) error {
			si++
			if si%2 == 0 {
				return fmt.Errorf("some-fake-error")
			}
			return nil
		}
	})

	// we should see intermittent failures, but no resets
	awaitEvents(t, testEvents,
		expect(syncExportErr, syncExportErr, exportSuccess, exportSuccess),
		deny(resetFromExport, resetFromRun, sentinelLost),
	)

	// remove intermittent sync failures
	driver.withLock(func() {
		driver.sync = nil
	})

	// drain remaining failures and ensure that we hit at least one success
	awaitEvents(t, testEvents,
		expect(exportSuccess),
		deny(resetFromExport, resetFromRun, sentinelLost),
		drain(true),
	)

	// introduce intermittent failure to the export fn
	exportLock.Lock()
	exportFlaky = true
	exportLock.Unlock()

	// we should see intermittent failures, but no resets
	awaitEvents(t, testEvents,
		expect(getExportErr, getExportErr, exportSuccess, exportSuccess),
		deny(resetFromExport, resetFromRun, sentinelLost),
	)

	// introduce persistent failure to the export fn
	exportLock.Lock()
	exportFlaky = false
	exportFail = true
	exportLock.Unlock()

	// drain remaining successes and wait for next failure
	awaitEvents(t, testEvents,
		expect(getExportErr),
		deny(resetFromRun, sentinelLost),
		drain(true),
	)

	// ensure that we now observe frequent resets and no successes
	awaitEvents(t, testEvents,
		expect(resetFromExport, resetFromExport),
		deny(resetFromRun, sentinelLost, exportSuccess),
	)

	// clear export fail state
	exportLock.Lock()
	exportFail = false
	exportLock.Unlock()

	// terminate our first connection sentinel
	s1Cancel()

	// wait until we lose the sentinel
	awaitEvents(t, testEvents,
		expect(sentinelLost),
	)

	// we should revert to periodic resets
	awaitEvents(t, testEvents,
		expect(resetFromRun),
		deny(sentinelAcquired, exportAttempt),
	)

	// provide another sentinel
	s2, s2Cancel := context.WithCancel(ctx)
	sc <- s2

	// healthy operation should resume
	awaitEvents(t, testEvents,
		expect(sentinelAcquired, exportSuccess),
		deny(resetFromExport, exportFailure),
	)

	s2Cancel()
}

type eventOpts struct {
	expect map[testEvent]int
	deny   map[testEvent]struct{}
	drain  bool
}

type eventOption func(*eventOpts)

func expect(events ...testEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.expect[event] = opts.expect[event] + 1
		}
	}
}

func deny(events ...testEvent) eventOption {
	return func(opts *eventOpts) {
		for _, event := range events {
			opts.deny[event] = struct{}{}
		}
	}
}

func drain(d bool) eventOption {
	return func(opts *eventOpts) {
		opts.drain = d
	}
}

func awaitEvents(t *testing.T, ch <-chan testEvent, opts ...eventOption) {
	options := eventOpts{
		expect: make(map[testEvent]int),
		deny:   make(map[testEvent]struct{}),
	}
	for _, opt := range opts {
		opt(&options)
	}

	if options.drain {
		drainEvents(t, ch, options)
	}

	timeout := time.After(time.Second * 5)
	for {
		if len(options.expect) == 0 {
			return
		}

		select {
		case event := <-ch:
			if _, ok := options.deny[event]; ok {
				require.Failf(t, "unexpected event", "event=%v", event)
			}

			options.expect[event] = options.expect[event] - 1
			if options.expect[event] < 1 {
				delete(options.expect, event)
			}
		case <-timeout:
			require.Failf(t, "timeout waiting for events", "expect=%+v", options.expect)
		}
	}
}

func drainEvents(t *testing.T, ch <-chan testEvent, options eventOpts) {
	timeout := time.After(time.Second * 5)
	for {
		select {
		case event := <-ch:
			if _, ok := options.deny[event]; ok {
				require.Failf(t, "unexpected event", "event=%v", event)
			}
		case <-timeout:
			require.Fail(t, "timeout attempting to drain events channel")
		default:
			return
		}
	}
}
