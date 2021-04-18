// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

func TestNopLogger_Printf(t *testing.T) {
	l := NopLogger{}
	l.Printf("foo")
	l.Printf("bar['%s']='%v'", "baz", "qux")
}

type mockLogger struct {
	mock.Mock
}

func newMockLogger(t *testing.T) *mockLogger {
	m := &mockLogger{}
	m.Test(t)
	return m
}

func (m *mockLogger) Printf(f string, a ...interface{}) {
	if a == nil {
		m.Called(f)
	} else {
		m.Called(f, a)
	}
}
