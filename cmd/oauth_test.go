// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"launchpad.net/gocheck"
	"os"
)

func (s *S) TestClientID(c *gocheck.C) {
	err := os.Setenv("TSURU_AUTH_CLIENTID", "someid")
	c.Assert(err, gocheck.IsNil)
	c.Assert("someid", gocheck.Equals, clientID())
}

func (s *S) TestPort(c *gocheck.C) {
	err := os.Setenv("TSURU_AUTH_SERVER_PORT", ":4242")
	c.Assert(err, gocheck.IsNil)
	c.Assert(":4242", gocheck.Equals, port())
	err = os.Setenv("TSURU_AUTH_SERVER_PORT", "")
	c.Assert(err, gocheck.IsNil)
	c.Assert(":0", gocheck.Equals, port())
}
