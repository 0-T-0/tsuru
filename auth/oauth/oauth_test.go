// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package oauth

import (
	"launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { gocheck.TestingT(t) }

type S struct{}

var _ = gocheck.Suite(&S{})

func (s *S) TestOAuthLoginWithoutCode(c *gocheck.C) {
	scheme := OAuthScheme{}
	params := make(map[string]string)
	_, err := scheme.Login(params)
	c.Assert(err, gocheck.Equals, ErrMissingCodeError)
}
