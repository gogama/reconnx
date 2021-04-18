// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import (
	"math"
	"sync"
)

const (
	DefaultHistoryLen = 10
	DefaultRecentLen  = 3
	badWindowSizeMsg  = "reconnx: window size must be positive"
)

type State int

const (
	Watching State = iota
	Closing
	Resting
)

func (s State) String() string {
	switch s {
	case Watching:
		return "Watching"
	case Closing:
		return "Closing"
	case Resting:
		return "Resting"
	default:
		return ""
	}
}

type Machine interface {
	State() State
	Next(value float64, closed bool) (next State, prev State)
}

type avgWindow struct {
	values []float64
	sum    float64
	index  int
}

func newAvgWindow(n uint, def float64) avgWindow {
	if n < 1 {
		panic("reconnx: window size must be positive")
	}
	values := make([]float64, n)
	sum := 0.0
	for i := uint(0); i < n; i++ {
		values[i] = def
		sum += def
	}
	return avgWindow{values: values, sum: sum}
}

func (aw *avgWindow) Avg() float64 {
	return aw.sum / float64(len(aw.values))
}

func (aw *avgWindow) Push(value float64) (dropped float64) {
	dropped = aw.values[aw.index]
	aw.values[aw.index] = value
	aw.index++
	aw.sum -= dropped
	aw.sum += value
	if aw.index >= len(aw.values) {
		aw.index = 0
	}
	return
}

type machine struct {
	state        State
	history      avgWindow
	recent       avgWindow
	closedStreak uint
	closedCount  uint
	restCount    uint
	config       MachineConfig
	lock         sync.RWMutex
}

type MachineConfig struct {
	HistoryLen    uint
	RecentLen     uint
	AbsoluteMax   float64
	PercentMax    float64
	ClosingStreak uint
	ClosingMax    uint
	RestingMax    uint
}

func NewMachine(config MachineConfig) Machine {
	historyLen := valOrDef(config.HistoryLen, DefaultHistoryLen)
	recentLen := valOrDef(config.RecentLen, DefaultRecentLen)
	return &machine{
		history: newAvgWindow(historyLen, config.AbsoluteMax),
		recent:  newAvgWindow(recentLen, config.AbsoluteMax),
		config:  config,
	}
}

func (m *machine) State() State {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.state
}

func (m *machine) Next(value float64, closed bool) (next State, prev State) {
	m.lock.Lock()
	defer m.lock.Unlock()

	prev = m.state
	m.shift(value)
	switch prev {
	case Watching:
		m.watching()
	case Closing:
		m.closing(closed)
	case Resting:
		m.resting()
	default:
		panic("reconnx: unknown state")
	}

	next = m.state
	return
}

func (m *machine) shift(value float64) {
	if m.config.AbsoluteMax > 0.0 {
		value = math.Min(value, m.config.AbsoluteMax)
	}
	dropped := m.recent.Push(value)
	m.history.Push(dropped)
}

func (m *machine) watching() {
	recentAvg := m.recent.Avg()
	if m.config.AbsoluteMax > 0.0 && recentAvg >= m.config.AbsoluteMax {
		m.state = Closing
	} else if m.config.PercentMax > 0.0 && recentAvg >= m.history.Avg()*(1.0+m.config.PercentMax) {
		m.state = Closing
	}

	if m.state == Closing {
		m.closing(false)
	}
}

func (m *machine) closing(closed bool) {
	if closed {
		m.closedCount++
		m.closedStreak++
	} else {
		m.closedStreak = 0
	}
	if m.closedStreak >= m.config.ClosingStreak || m.closedCount >= m.config.ClosingMax {
		m.closedCount = 0
		m.closedStreak = 0
		if m.config.RestingMax > 0 {
			m.state = Resting
		} else {
			m.state = Watching
		}
	}
}

func (m *machine) resting() {
	m.restCount++
	if m.restCount >= m.config.RestingMax {
		m.restCount = 0
		m.state = Watching
		m.watching()
	}
}

func valOrDef(val, def uint) uint {
	if val > 0 {
		return val
	}

	return def
}
