// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"launchpad.net/gocheck"
)

func (s *S) TestPlanAdd(c *gocheck.C) {
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"name": "xyz", "memory": 9223372036854775807, "swap": 1024, "cpushare": 100 }`)
	request, err := http.NewRequest("POST", "/plans", body)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m := RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusCreated)
	defer s.conn.Plans().RemoveAll(nil)
	var plans []Plan
	err = s.conn.Plans().Find(nil).All(&plans)
	c.Assert(err, gocheck.IsNil)
	c.Assert(plans, gocheck.DeepEquals, []Plan{
		{Name: "xyz", Memory: 9223372036854775807, Swap: 1024, CpuShare: 100},
	})
}

func (s *S) TestPlanAddInvalidJson(c *gocheck.C) {
	recorder := httptest.NewRecorder()
	body := strings.NewReader(`{"name": "", "memory": 9223372036854775807, "swap": 1024, "cpushare": 100}`)
	request, err := http.NewRequest("POST", "/plans", body)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m := RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	body = strings.NewReader(`{"name": "xxx", "memory": 0, "swap": 1024, "cpushare": 100}`)
	request, err = http.NewRequest("POST", "/plans", body)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m = RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	body = strings.NewReader(`{"name": "xxx", "memory": 1234, "swap": 0, "cpushare": 100}`)
	request, err = http.NewRequest("POST", "/plans", body)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m = RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	body = strings.NewReader(`{"name": "xxx", "memory": 1234, "swap": 9999, "cpushare": 0}`)
	request, err = http.NewRequest("POST", "/plans", body)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m = RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)

	recorder = httptest.NewRecorder()
	body = strings.NewReader(`{"name": "xxx", ".........`)
	request, err = http.NewRequest("POST", "/plans", body)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m = RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusBadRequest)
}

func (s *S) TestPlanList(c *gocheck.C) {
	recorder := httptest.NewRecorder()
	expected := []Plan{
		{Name: "plan1", Memory: 1, Swap: 2, CpuShare: 3},
		{Name: "plan2", Memory: 3, Swap: 4, CpuShare: 5},
	}
	err := s.conn.Plans().Insert(expected[0])
	c.Assert(err, gocheck.IsNil)
	err = s.conn.Plans().Insert(expected[1])
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Plans().RemoveAll(nil)
	request, err := http.NewRequest("GET", "/plans", nil)
	c.Assert(err, gocheck.IsNil)
	request.Header.Set("Authorization", "bearer "+s.admintoken.GetValue())
	m := RunServer(true)
	m.ServeHTTP(recorder, request)
	c.Assert(recorder.Code, gocheck.Equals, http.StatusOK)
	var plans []Plan
	err = json.Unmarshal(recorder.Body.Bytes(), &plans)
	c.Assert(err, gocheck.IsNil)
	c.Assert(plans, gocheck.DeepEquals, expected)
}
