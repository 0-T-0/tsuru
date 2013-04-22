// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package log provides logging utility.
//
// It abstracts the logger from the standard log package, allowing the
// developer to patck the logging target, changing this to a file, or syslog,
// for example.
package log

import (
	"io"
	"log"
	"sync"
)

// Target is the current target for the log package.
type Target struct {
	logger *log.Logger
	mut    sync.RWMutex
}

// SetLogger defines a new logger for the current target.
//
// See the builtin log package for more details.
func (t *Target) SetLogger(l *log.Logger) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.logger = l
}

// Fatal is equivalent to Print() followed by os.Exit(1).
func (t *Target) Fatal(v ...interface{}) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	if t.logger != nil {
		t.logger.Fatal(v...)
	}
}

// Fatalf is equivalent to Printf followed by os.Exit(1).
func (t *Target) Fatalf(format string, v ...interface{}) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	if t.logger != nil {
		t.logger.Fatalf(format, v...)
	}
}

// Print is similar to fmt.Print, writing the given values to the Target
// logger.
func (t *Target) Print(v ...interface{}) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	if t.logger != nil {
		t.logger.Print(v...)
	}
}

// Printf is similar to fmt.Printf, writing the formatted string to the Target
// logger.
func (t *Target) Printf(format string, v ...interface{}) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	if t.logger != nil {
		t.logger.Printf(format, v...)
	}
}

// Panic is equivalent to Print() followed by panic().
func (t *Target) Panic(v ...interface{}) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	if t.logger != nil {
		t.logger.Panic(v...)
	}
}

func (t *Target) Panicf(format string, v ...interface{}) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	if t.logger != nil {
		t.logger.Panicf(format, v...)
	}
}

var DefaultTarget *Target = new(Target)

// Fatal is a wrapper for DefaultTarget.Fatal.
func Fatal(v ...interface{}) {
	DefaultTarget.Fatal(v...)
}

// Fatalf is a wrapper for DefaultTarget.Fatalf.
func Fatalf(format string, v ...interface{}) {
	DefaultTarget.Fatalf(format, v...)
}

// Print is a wrapper for DefaultTarget.Print.
func Print(v ...interface{}) {
	DefaultTarget.Print(v...)
}

// Printf is a wrapper for DefaultTarget.Printf.
func Printf(format string, v ...interface{}) {
	DefaultTarget.Printf(format, v...)
}

// Panic is a wrapper for DefaultTarget.Panic.
func Panic(v ...interface{}) {
	DefaultTarget.Panic(v...)
}

// Panicf is a wrapper for DefaultTarget.Panicf.
func Panicf(format string, v ...interface{}) {
	DefaultTarget.Panicf(format, v...)
}

// SetLogger is a wrapper for DefaultTarget.SetLogger.
func SetLogger(logger *log.Logger) {
	DefaultTarget.SetLogger(logger)
}

func Write(w io.Writer, content []byte) error {
	n, err := w.Write(content)
	if err != nil {
		return err
	}
	if n != len(content) {
		return io.ErrShortWrite
	}
	return nil
}
