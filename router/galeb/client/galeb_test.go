// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ttesting "github.com/tsuru/tsuru/testing"
	"launchpad.net/gocheck"
)

func Test(t *testing.T) {
	gocheck.TestingT(t)
}

type S struct {
	server  *httptest.Server
	handler ttesting.MultiTestHandler
	client  *GalebClient
}

var _ = gocheck.Suite(&S{})

func (s *S) SetUpTest(c *gocheck.C) {
	s.handler = ttesting.MultiTestHandler{}
	s.server = httptest.NewServer(&s.handler)
	s.client = &GalebClient{
		ApiUrl:   s.server.URL + "/api",
		Username: "myusername",
		Password: "mypassword",
	}
}

func (s *S) TearDownTest(c *gocheck.C) {
	s.server.Close()
}

func (s *S) TestNewGalebClient(c *gocheck.C) {
	c.Assert(s.client.ApiUrl, gocheck.Equals, s.server.URL+"/api")
	c.Assert(s.client.Username, gocheck.Equals, "myusername")
	c.Assert(s.client.Password, gocheck.Equals, "mypassword")
}

func (s *S) TestGalebAddBackendPool(c *gocheck.C) {
	s.handler.Content = `{
      "_links": {
        "self": "http://galeb.somewhere/api/backendpool/3/"
      },
      "id": 3,
      "name": "pool2",
      "environment": "http://galeb.somewhere/api/environment/1/",
      "farmtype": "http://galeb.somewhere/api/farmtype/1/",
      "plan": "http://galeb.somewhere/api/plan/1/",
      "project": "http://galeb.somewhere/api/project/3/",
      "loadbalancepolicy": "http://galeb.somewhere/api/loadbalancepolicy/1/",
      "status": "201"
    }`
	s.handler.RspCode = http.StatusCreated
	params := BackendPoolParams{
		Name:              "myname",
		Environment:       "myenv",
		Plan:              "myplan",
		Project:           "myproject",
		LoadBalancePolicy: "mypolicy",
		FarmType:          "mytype",
	}
	fullId, err := s.client.AddBackendPool(&params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(s.handler.Method, gocheck.DeepEquals, []string{"POST"})
	c.Assert(s.handler.Url, gocheck.DeepEquals, []string{"/api/backendpool/"})
	var parsedParams BackendPoolParams
	err = json.Unmarshal(s.handler.Body[0], &parsedParams)
	c.Assert(err, gocheck.IsNil)
	c.Assert(parsedParams, gocheck.DeepEquals, params)
	c.Assert(s.handler.Header[0].Get("Content-Type"), gocheck.Equals, "application/json")
	c.Assert(fullId, gocheck.Equals, "http://galeb.somewhere/api/backendpool/3/")
}

func (s *S) TestGalebAddBackendPoolInvalidStatusCode(c *gocheck.C) {
	s.handler.RspCode = http.StatusOK
	s.handler.Content = "invalid content"
	params := BackendPoolParams{}
	fullId, err := s.client.AddBackendPool(&params)
	c.Assert(err, gocheck.ErrorMatches,
		"POST /backendpool/: invalid response code: 200: invalid content - PARAMS: .+")
	c.Assert(fullId, gocheck.Equals, "")
}

func (s *S) TestGalebAddBackendPoolInvalidResponse(c *gocheck.C) {
	s.handler.RspCode = http.StatusCreated
	s.handler.Content = "invalid content"
	params := BackendPoolParams{}
	fullId, err := s.client.AddBackendPool(&params)
	c.Assert(err, gocheck.ErrorMatches,
		"POST /backendpool/: unable to parse response: invalid content: invalid character 'i' looking for beginning of value - PARAMS: .+")
	c.Assert(fullId, gocheck.Equals, "")
}

func (s *S) TestGalebAddBackendPoolDefaultValues(c *gocheck.C) {
	s.client.Environment = "env1"
	s.client.FarmType = "type1"
	s.client.Plan = "plan1"
	s.client.Project = "project1"
	s.client.LoadBalancePolicy = "policy1"
	s.handler.RspCode = http.StatusCreated
	s.handler.Content = `{
      "_links": {
        "self": "http://galeb.somewhere/api/backendpool/999/"
      }
    }`
	c.Assert(s.client.Environment, gocheck.Equals, "env1")
	c.Assert(s.client.FarmType, gocheck.Equals, "type1")
	c.Assert(s.client.Plan, gocheck.Equals, "plan1")
	c.Assert(s.client.Project, gocheck.Equals, "project1")
	c.Assert(s.client.LoadBalancePolicy, gocheck.Equals, "policy1")
	params := BackendPoolParams{Name: "mypool"}
	fullId, err := s.client.AddBackendPool(&params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(fullId, gocheck.Equals, "http://galeb.somewhere/api/backendpool/999/")
	var parsedParams BackendPoolParams
	err = json.Unmarshal(s.handler.Body[0], &parsedParams)
	c.Assert(err, gocheck.IsNil)
	expected := BackendPoolParams{
		Name:              "mypool",
		Environment:       "env1",
		Plan:              "plan1",
		Project:           "project1",
		LoadBalancePolicy: "policy1",
		FarmType:          "type1",
	}
	c.Assert(parsedParams, gocheck.DeepEquals, expected)
}

func (s *S) TestGalebAddBackend(c *gocheck.C) {
	s.handler.Content = `{
      "_links": {
        "self": "http://galeb.somewhere/api/backend/9/"
      },
      "id": 9,
      "ip": "10.0.0.1",
      "port": 8080,
      "backendpool": "http://galeb.somewhere/api/backendpool/1/",
      "status": "201"
    }`
	s.handler.RspCode = http.StatusCreated
	params := BackendParams{
		Ip:          "10.0.0.1",
		Port:        8080,
		BackendPool: "http://galeb.somewhere/api/backendpool/1/",
	}
	fullId, err := s.client.AddBackend(&params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(s.handler.Method, gocheck.DeepEquals, []string{"POST"})
	c.Assert(s.handler.Url, gocheck.DeepEquals, []string{"/api/backend/"})
	var parsedParams BackendParams
	err = json.Unmarshal(s.handler.Body[0], &parsedParams)
	c.Assert(err, gocheck.IsNil)
	c.Assert(parsedParams, gocheck.DeepEquals, params)
	c.Assert(s.handler.Header[0].Get("Content-Type"), gocheck.Equals, "application/json")
	c.Assert(fullId, gocheck.Equals, "http://galeb.somewhere/api/backend/9/")
}

func (s *S) TestGalebAddRuleDefaultValues(c *gocheck.C) {
	s.client.RuleType = "rule1"
	s.client.Project = "project1"
	s.handler.RspCode = http.StatusCreated
	s.handler.Content = `{
      "_links": {
        "self": "http://galeb.somewhere/api/rule/999/"
      }
    }`
	c.Assert(s.client.RuleType, gocheck.Equals, "rule1")
	c.Assert(s.client.Project, gocheck.Equals, "project1")
	params := RuleParams{
		Name:        "myrule",
		Match:       "/",
		BackendPool: "pool1",
	}
	fullId, err := s.client.AddRule(&params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(fullId, gocheck.Equals, "http://galeb.somewhere/api/rule/999/")
	var parsedParams RuleParams
	err = json.Unmarshal(s.handler.Body[0], &parsedParams)
	c.Assert(err, gocheck.IsNil)
	expected := RuleParams{
		Name:        "myrule",
		Match:       "/",
		BackendPool: "pool1",
		RuleType:    "rule1",
		Project:     "project1",
	}
	c.Assert(parsedParams, gocheck.DeepEquals, expected)
}

func (s *S) TestGalebAddVirtualHostDefaultValues(c *gocheck.C) {
	s.client.FarmType = "farm1"
	s.client.Plan = "plan1"
	s.client.Environment = "env1"
	s.client.Project = "project1"
	s.handler.RspCode = http.StatusCreated
	s.handler.Content = `{
      "_links": {
        "self": "http://galeb.somewhere/api/virtualhost/999/"
      }
    }`
	c.Assert(s.client.FarmType, gocheck.Equals, "farm1")
	c.Assert(s.client.Project, gocheck.Equals, "project1")
	c.Assert(s.client.Plan, gocheck.Equals, "plan1")
	c.Assert(s.client.Environment, gocheck.Equals, "env1")
	params := VirtualHostParams{
		Name:        "myvirtualhost.com",
		RuleDefault: "myrule",
	}
	fullId, err := s.client.AddVirtualHost(&params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(fullId, gocheck.Equals, "http://galeb.somewhere/api/virtualhost/999/")
	var parsedParams VirtualHostParams
	err = json.Unmarshal(s.handler.Body[0], &parsedParams)
	c.Assert(err, gocheck.IsNil)
	expected := VirtualHostParams{
		Name:        "myvirtualhost.com",
		RuleDefault: "myrule",
		FarmType:    "farm1",
		Plan:        "plan1",
		Environment: "env1",
		Project:     "project1",
	}
	c.Assert(parsedParams, gocheck.DeepEquals, expected)
}

func (s *S) TestGalebAddVirtualHostRule(c *gocheck.C) {
	s.handler.Content = `{
      "_links": {
        "self": "http://galeb.somewhere/api/virtualhostrule/9/"
      },
      "status": "201"
    }`
	s.handler.RspCode = http.StatusCreated
	params := VirtualHostRuleParams{
		Order:       1,
		Rule:        "rule1",
		VirtualHost: "virtualhost1",
	}
	fullId, err := s.client.AddVirtualHostRule(&params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(s.handler.Method, gocheck.DeepEquals, []string{"POST"})
	c.Assert(s.handler.Url, gocheck.DeepEquals, []string{"/api/virtualhostrule/"})
	var parsedParams VirtualHostRuleParams
	err = json.Unmarshal(s.handler.Body[0], &parsedParams)
	c.Assert(err, gocheck.IsNil)
	c.Assert(parsedParams, gocheck.DeepEquals, params)
	c.Assert(s.handler.Header[0].Get("Content-Type"), gocheck.Equals, "application/json")
	c.Assert(fullId, gocheck.Equals, "http://galeb.somewhere/api/virtualhostrule/9/")
}

func (s *S) TestGalebRemoveResource(c *gocheck.C) {
	s.handler.RspCode = http.StatusNoContent
	err := s.client.RemoveResource(s.client.ApiUrl + "/backendpool/10/")
	c.Assert(err, gocheck.IsNil)
	c.Assert(s.handler.Method, gocheck.DeepEquals, []string{"DELETE"})
	c.Assert(s.handler.Url, gocheck.DeepEquals, []string{"/api/backendpool/10/"})
}

func (s *S) TestGalebRemoveResourceInvalidResponse(c *gocheck.C) {
	s.handler.RspCode = http.StatusOK
	s.handler.Content = "invalid content"
	err := s.client.RemoveResource(s.client.ApiUrl + "/backendpool/10/")
	c.Assert(err, gocheck.ErrorMatches, "DELETE /backendpool/10/: invalid response code: 200: invalid content")
}
