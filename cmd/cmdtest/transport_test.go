// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"io/ioutil"
	"net/http"
	"testing"

	"launchpad.net/gocheck"
)

type S struct{}

var _ = gocheck.Suite(S{})

func Test(t *testing.T) {
	gocheck.TestingT(t)
}

func (S) TestTransport(c *gocheck.C) {
	var t http.RoundTripper = &Transport{
		Message: "Ok",
		Status:  http.StatusOK,
		Headers: map[string][]string{"Authorization": {"something"}},
	}
	req, _ := http.NewRequest("GET", "/", nil)
	r, err := t.RoundTrip(req)
	c.Assert(err, gocheck.IsNil)
	c.Assert(r.StatusCode, gocheck.Equals, http.StatusOK)
	defer r.Body.Close()
	b, _ := ioutil.ReadAll(r.Body)
	c.Assert(string(b), gocheck.Equals, "Ok")
	c.Assert(r.Header.Get("Authorization"), gocheck.Equals, "something")
}

func (S) TestConditionalTransport(c *gocheck.C) {
	var t http.RoundTripper = &ConditionalTransport{
		Transport: Transport{
			Message: "Ok",
			Status:  http.StatusOK,
		},
		CondFunc: func(req *http.Request) bool {
			return req.URL.Path == "/something"
		},
	}
	req, _ := http.NewRequest("GET", "/something", nil)
	r, err := t.RoundTrip(req)
	c.Assert(err, gocheck.IsNil)
	c.Assert(r.StatusCode, gocheck.Equals, http.StatusOK)
	defer r.Body.Close()
	b, _ := ioutil.ReadAll(r.Body)
	c.Assert(string(b), gocheck.Equals, "Ok")
	req, _ = http.NewRequest("GET", "/", nil)
	r, err = t.RoundTrip(req)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "condition failed")
	c.Assert(r.StatusCode, gocheck.Equals, http.StatusInternalServerError)
}

func (S) TestMultiConditionalTransport(c *gocheck.C) {
	t1 := ConditionalTransport{
		Transport: Transport{
			Message: "Unauthorized",
			Status:  http.StatusUnauthorized,
		},
		CondFunc: func(req *http.Request) bool {
			return req.URL.Path == "/something"
		},
	}
	t2 := ConditionalTransport{
		Transport: Transport{
			Message: "OK",
			Status:  http.StatusOK,
		},
		CondFunc: func(req *http.Request) bool {
			return req.URL.Path == "/something"
		},
	}
	m := MultiConditionalTransport{
		ConditionalTransports: []ConditionalTransport{t1, t2},
	}
	c.Assert(len(m.ConditionalTransports), gocheck.Equals, 2)
	req, _ := http.NewRequest("GET", "/something", nil)
	r, err := m.RoundTrip(req)
	c.Assert(err, gocheck.IsNil)
	c.Assert(r.StatusCode, gocheck.Equals, http.StatusUnauthorized)
	c.Assert(len(m.ConditionalTransports), gocheck.Equals, 1)
	r, err = m.RoundTrip(req)
	c.Assert(err, gocheck.IsNil)
	c.Assert(r.StatusCode, gocheck.Equals, http.StatusOK)
	c.Assert(len(m.ConditionalTransports), gocheck.Equals, 0)
}
