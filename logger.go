// Copyright 2021 The reconnx Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package reconnx

// Logger allows the reconnx plugin to log state changes and other
// information about plugin operation. The interface is compatible with
// the Go standard log.Logger.
//
// Implementations of Logger must be safe for concurrent use by multiple
// goroutines.
type Logger interface {
	// Printf prints a message to the Logger. Arguments are handled in
	// the manner of fmt.Printf.
	Printf(format string, v ...interface{})
}

// NopLogger implements the Logger interface but ignores all messages
// sent to it. Use NopLogger if you are not interested in receiving
// updates about plugin operation.
type NopLogger struct{}

func (NopLogger) Printf(string, ...interface{}) {
}
