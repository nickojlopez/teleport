/*
Copyright 2022 Gravitational, Inc.

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

package types

import (
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/trace"
)

const (
	UpgraderKindKubeController = "kube"
	UpgraderKindSystemdUnit    = "unit"
)

var validWeekdays = [7]time.Weekday{
	time.Sunday,
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
}

// parseWeekday attempts to interpret a string as a time.Weekday. In the interest of flexibility,
// parsing is case-insensitive and supports the common three-letter shorthand accepted by many
// common scheduling utilites (e.g. contab, systemd timers).
func parseWeekday(s string) (day time.Weekday, ok bool) {
	for _, w := range validWeekdays {
		if strings.EqualFold(w.String(), s) || strings.EqualFold(w.String()[:3], s) {
			return w, true
		}
	}

	return time.Sunday, false
}

// generator builds a closure that iterates valid maintenance window from the current day onward. Used in
// schedule export logic and tests.
func (w *AgentUpgradeWindow) generator(from time.Time) func() (start time.Time, end time.Time) {
	from = from.UTC()
	next := time.Date(
		from.Year(),
		from.Month(),
		from.Day(),
		int(w.UTCStartHour%24),
		0, // min
		0, // sec
		0, // nsec
		time.UTC,
	)

	var weekdays []time.Weekday
	for _, d := range w.Weekdays {
		if p, ok := parseWeekday(d); ok {
			weekdays = append(weekdays, p)
		}
	}

	return func() (start time.Time, end time.Time) {
		for { // safe because invalid weekdays have been filtered out
			start = next
			end = start.Add(time.Hour)

			next = next.AddDate(0, 0, 1)

			if len(weekdays) == 0 {
				return
			}

			for _, day := range weekdays {
				if start.Weekday() == day {
					return
				}
			}
		}
	}
}

// Export exports the next `n` upgrade windows as a schedule object, starting from `from`.
func (w *AgentUpgradeWindow) Export(from time.Time, n int) AgentUpgradeSchedule {
	gen := w.generator(from)

	sched := AgentUpgradeSchedule{
		Windows: make([]ScheduledAgentUpgradeWindow, 0, n),
	}
	for i := 0; i < n; i++ {
		start, stop := gen()
		sched.Windows = append(sched.Windows, ScheduledAgentUpgradeWindow{
			Start: start.UTC(),
			Stop:  stop.UTC(),
		})
	}

	return sched
}

func (s *AgentUpgradeSchedule) Clone() *AgentUpgradeSchedule {
	return proto.Clone(s).(*AgentUpgradeSchedule)
}

// NewMaintenanceWindow creates a new maintenance window with no parameters set.
func NewMaintenanceWindow() MaintenanceWindow {
	var mw MaintenanceWindowV1
	mw.setStaticFields()
	return &mw
}

type MaintenanceWindow interface {
	Resource

	// GetNonce gets the nonce of the maintenance window.
	GetNonce() uint64

	// WithNonce creates a shallow copy with a new nonce.
	WithNonce(nonce uint64) any

	// GetAgentUpgradeWindow gets the agent upgrade window.
	GetAgentUpgradeWindow() (win AgentUpgradeWindow, ok bool)

	// SetAgentUpgradeWindow sets the agent upgrade window.
	SetAgentUpgradeWindow(win AgentUpgradeWindow)

	CheckAndSetDefaults() error
}

func (m *MaintenanceWindowV1) setStaticFields() {
	if m.Version == "" {
		m.Version = V1
	}

	if m.Kind == "" {
		m.Kind = KindMaintenanceWindow
	}

	if m.Metadata.Name == "" {
		m.Metadata.Name = MetaNameMaintenanceWindow
	}
}

func (m *MaintenanceWindowV1) CheckAndSetDefaults() error {
	m.setStaticFields()

	if err := m.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if m.Version != V1 {
		return trace.BadParameter("unexpected maintenance window resource version %q (expected %q)", m.Version, V1)
	}

	if m.Kind == MetaNameMaintenanceWindow {
		// normalize easy mixup
		m.Kind = KindMaintenanceWindow
	}

	if m.Kind != KindMaintenanceWindow {
		return trace.BadParameter("unexpected maintenance window kind %q (expected %q)", m.Kind, KindMaintenanceWindow)
	}

	if m.Metadata.Name == KindMaintenanceWindow {
		// normalize easy mixup
		m.Metadata.Name = MetaNameMaintenanceWindow
	}

	if m.Metadata.Name != MetaNameMaintenanceWindow {
		return trace.BadParameter("unexpected maintenance window name %q (expected %q)", m.Metadata.Name, MetaNameMaintenanceWindow)
	}

	if m.Spec.AgentUpgrades != nil {
		if h := m.Spec.AgentUpgrades.UTCStartHour; h > 23 {
			return trace.BadParameter("agent upgrade window utc start hour must be in range 0..23, got %d", h)
		}

		for _, day := range m.Spec.AgentUpgrades.Weekdays {
			if _, ok := parseWeekday(day); !ok {
				return trace.BadParameter("invalid weekday in agent upgrade window: %q", day)
			}
		}
	}

	return nil
}

func (m *MaintenanceWindowV1) GetNonce() uint64 {
	return m.Spec.Nonce
}

func (m *MaintenanceWindowV1) WithNonce(nonce uint64) any {
	shallowCopy := *m
	shallowCopy.Spec.Nonce = nonce
	return &shallowCopy
}

func (m *MaintenanceWindowV1) GetAgentUpgradeWindow() (win AgentUpgradeWindow, ok bool) {
	if m.Spec.AgentUpgrades == nil {
		return AgentUpgradeWindow{}, false
	}

	return *m.Spec.AgentUpgrades, true
}

func (m *MaintenanceWindowV1) SetAgentUpgradeWindow(win AgentUpgradeWindow) {
	m.Spec.AgentUpgrades = &win
}
