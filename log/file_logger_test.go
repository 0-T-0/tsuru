// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package log

import (
	"bytes"
	"launchpad.net/gocheck"
	"log"
)

type FileLoggerSuite struct{}

var _ = gocheck.Suite(&FileLoggerSuite{})

func (s *FileLoggerSuite) TestNewFileLoggerReturnsALogger(c *gocheck.C) {
	l := newFileLogger("/dev/null")
	_, ok := l.(Logger)
	c.Assert(ok, gocheck.Equals, true)
}

func (s *FileLoggerSuite) TestNewFileLoggerInitializesWriter(c *gocheck.C) {
	l := newFileLogger("/dev/null")
	fl, _ := l.(*fileLogger)
	c.Assert(fl.logger, gocheck.FitsTypeOf, &log.Logger{})
}

func (s *FileLoggerSuite) TestErrorShouldPrefixMessage(c *gocheck.C) {
	l := newFileLogger("/dev/null")
	fl, _ := l.(*fileLogger)
	b := &bytes.Buffer{}
	fl.logger = log.New(b, "", log.LstdFlags)
	l.Error("something terrible happened")
	c.Assert(b.String(), gocheck.Matches, ".* ERROR: something terrible happened\n$")
}

func (s *FileLoggerSuite) TestErrorfShouldFormatErrorAndPrefixMessage(c *gocheck.C) {
	l := newFileLogger("/dev/null")
	fl, _ := l.(*fileLogger)
	b := &bytes.Buffer{}
	fl.logger = log.New(b, "", log.LstdFlags)
	l.Errorf(`this is the error: "%s"`, "something bad happened")
	c.Assert(b.String(), gocheck.Matches, `.* ERROR: this is the error: "something bad happened"\n$`)
}

func (s *FileLoggerSuite) TestDebugShouldPrefixMessage(c *gocheck.C) {
	l := newFileLogger("/dev/null")
	fl, _ := l.(*fileLogger)
	b := &bytes.Buffer{}
	fl.logger = log.New(b, "", log.LstdFlags)
	l.Debug("doing some stuff here")
	c.Assert(b.String(), gocheck.Matches, ".* DEBUG: doing some stuff here\n$")
}

func (s *FileLoggerSuite) TestDebugfShouldFormatAndPrefixMessage(c *gocheck.C) {
	l := newFileLogger("/dev/null")
	fl, _ := l.(*fileLogger)
	b := &bytes.Buffer{}
	fl.logger = log.New(b, "", log.LstdFlags)
	l.Debugf(`message is "%s"`, "some debug message")
	c.Assert(b.String(), gocheck.Matches, `.* DEBUG: message is "some debug message"\n$`)
}
