// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudstack

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tsuru/config"
	"github.com/tsuru/tsuru/iaas"
	"launchpad.net/gocheck"
)

func Test(t *testing.T) { gocheck.TestingT(t) }

type cloudstackSuite struct{}

var _ = gocheck.Suite(&cloudstackSuite{})

func (s *cloudstackSuite) SetUpSuite(c *gocheck.C) {
	config.Set("iaas:cloudstack:api-key", "test")
	config.Set("iaas:cloudstack:secret-key", "test")
	config.Set("iaas:cloudstack:url", "test")
}

func (s *cloudstackSuite) TestHealthChecker(c *gocheck.C) {
	var command string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"listzonesresponse":{"count":8,"zone":[]}}`)
		command = r.URL.Query().Get("command")
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	err := healthChecker()
	c.Assert(err, gocheck.IsNil)
	c.Assert(command, gocheck.Equals, "listZones")
}

func (s *cloudstackSuite) TestHealthCheckerFailure(c *gocheck.C) {
	var command string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"listzonesresponse":{"count":0,"zone":[]}}`)
		command = r.URL.Query().Get("command")
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	err := healthChecker()
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, `"cloudstack" - not enough zones available, want at least 1, got 0`)
	c.Assert(command, gocheck.Equals, "listZones")
}

func (s *cloudstackSuite) TestHealthCheckerCustom(c *gocheck.C) {
	var cmds []string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"listzonesresponse":{"count":8,"zone":[]}}`)
		cmds = append(cmds, r.URL.Query().Get("command"))
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	config.Set("iaas:custom:cs_qa:provider", "cloudstack")
	defer config.Unset("iaas:custom:cs_qa:url")
	defer config.Unset("iaas:custom:cs_qa:provider")
	defer config.Unset("iaas:custom:cs_qa")
	err := healthChecker()
	c.Assert(err, gocheck.IsNil)
	c.Assert(cmds, gocheck.DeepEquals, []string{"listZones", "listZones"})
}

func (s *cloudstackSuite) TestHealthCheckerCustomFailure(c *gocheck.C) {
	var command string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"listzonesresponse":{"count":-1,"zone":[]}}`)
		command = r.URL.Query().Get("command")
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	config.Set("iaas:custom:cs_qa:provider", "cloudstack")
	defer config.Unset("iaas:custom:cs_qa:url")
	defer config.Unset("iaas:custom:cs_qa:provider")
	defer config.Unset("iaas:custom:cs_qa")
	err := healthChecker()
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, `"cs_qa" - not enough zones available, want at least 1, got -1`)
	c.Assert(command, gocheck.Equals, "listZones")
}

func (s *cloudstackSuite) TestCreateMachine(c *gocheck.C) {
	var calls []string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("command")
		calls = append(calls, cmd)
		w.Header().Set("Content-type", "application/json")
		if cmd == "queryAsyncJobResult" {
			fmt.Fprintln(w, `{"queryasyncjobresultresponse": {"jobstatus": 1}}`)
		}
		if cmd == "deployVirtualMachine" {
			fmt.Fprintln(w, `{"deployvirtualmachineresponse": {"id": "0366ae09-0a77-4e2b-8595-3b749764a107", "jobid": "test"}}`)
		}
		if cmd == "listVirtualMachines" {
			json := `{ "listvirtualmachinesresponse" : { "count":1 ,"virtualmachine" : [  {"id":"0366ae09-0a77-4e2b-8595-3b749764a107","name":"vm-0366ae09-0a77-4e2b-8595-3b749764a107","projectid":"a98738c9-5acd-43e3-b1a1-972a3db5b196","project":"tsuru playground","domainid":"eec2dacf-9982-11e3-a2b8-eee0bc1594e0","domain":"ROOT","created":"2014-07-18T18:29:30-0300","state":"Stopped","haenable":false,"zoneid":"95046c6c-65b8-415f-99cb-0cff40dc5f9c","zonename":"RJOEBT0200BE","templateid":"99f66d4c-f923-46e5-aa7b-09a0b22ee747","templatename":"ubuntu-14.04-server-amd64","templatedisplaytext":"ubuntu 14.04 ( 3.13.0-24-generic )","passwordenabled":false,"serviceofferingid":"3ff651c8-a27f-4008-87d5-71636aaabbc6","serviceofferingname":"Medium","cpunumber":2,"cpuspeed":1800,"memory":8192,"guestosid":"eede1fdf-9982-11e3-a2b8-eee0bc1594e0","rootdeviceid":0,"rootdevicetype":"ROOT","securitygroup":[],"nic":[{"id":"40cd6225-9475-44a3-8288-d7a9a485d8ac","networkid":"18c20437-df18-4757-8435-1230248f955b","networkname":"PLAYGROUND_200BE","netmask":"255.255.255.0","gateway":"10.24.16.1","ipaddress":"10.24.16.241","isolationuri":"vlan://19","broadcasturi":"vlan://19","traffictype":"Guest","type":"Shared","isdefault":true,"macaddress":"06:54:7e:00:46:c6"}],"hypervisor":"XenServer","tags":[],"affinitygroup":[],"displayvm":true,"isdynamicallyscalable":true,"jobid":"82a574cc-43f2-440d-8774-e638065c37af","jobstatus":0} ] } }`
			fmt.Fprintln(w, json)
		}
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	cs := NewCloudstackIaaS()
	params := map[string]string{
		"projectid":         "val",
		"networkids":        "val",
		"templateid":        "val",
		"serviceofferingid": "val",
		"zoneid":            "val",
	}
	vm, err := cs.CreateMachine(params)
	c.Assert(err, gocheck.IsNil)
	c.Assert(vm, gocheck.NotNil)
	c.Assert(vm.Address, gocheck.Equals, "10.24.16.241")
	c.Assert(vm.Id, gocheck.Equals, "0366ae09-0a77-4e2b-8595-3b749764a107")
	c.Assert(calls, gocheck.DeepEquals, []string{"deployVirtualMachine", "queryAsyncJobResult", "listVirtualMachines"})
}

func (s *cloudstackSuite) TestCreateMachineAsyncFailure(c *gocheck.C) {
	var calls []string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("command")
		calls = append(calls, cmd)
		w.Header().Set("Content-type", "application/json")
		if cmd == "queryAsyncJobResult" {
			fmt.Fprintln(w, `{"queryasyncjobresultresponse": {"jobstatus": 2, "jobresult": "my weird error"}}`)
		}
		if cmd == "deployVirtualMachine" {
			fmt.Fprintln(w, `{"deployvirtualmachineresponse": {"id": "0366ae09-0a77-4e2b-8595-3b749764a107", "jobid": "test"}}`)
		}
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	cs := NewCloudstackIaaS()
	params := map[string]string{
		"projectid":         "val",
		"networkids":        "val",
		"templateid":        "val",
		"serviceofferingid": "val",
		"zoneid":            "val",
	}
	_, err := cs.CreateMachine(params)
	c.Assert(err, gocheck.ErrorMatches, ".*my weird error.*")
	c.Assert(calls, gocheck.DeepEquals, []string{"deployVirtualMachine", "queryAsyncJobResult"})
}

func (s *cloudstackSuite) TestCreateMachineValidateParams(c *gocheck.C) {
	cs := NewCloudstackIaaS()
	params := map[string]string{
		"name": "something",
	}
	_, err := cs.CreateMachine(params)
	c.Assert(err, gocheck.ErrorMatches, "param \"networkids\" is mandatory")
}

func (s *cloudstackSuite) TestBuildUrlToCloudstack(c *gocheck.C) {
	cs := NewCloudstackIaaS()
	params := map[string]string{"atest": "2"}
	urlBuilded, err := cs.buildUrl("commandTest", params)
	c.Assert(err, gocheck.IsNil)
	u, err := url.Parse(urlBuilded)
	c.Assert(err, gocheck.IsNil)
	q, err := url.ParseQuery(u.RawQuery)
	c.Assert(err, gocheck.IsNil)
	c.Assert(q["signature"], gocheck.NotNil)
	c.Assert(q["apiKey"], gocheck.NotNil)
	c.Assert(q["atest"], gocheck.NotNil)
	c.Assert(q["response"], gocheck.DeepEquals, []string{"json"})
	c.Assert(q["command"], gocheck.DeepEquals, []string{"commandTest"})
}

func (s *cloudstackSuite) TestDeleteMachine(c *gocheck.C) {
	var calls []string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("command")
		calls = append(calls, cmd)
		w.Header().Set("Content-type", "application/json")
		if cmd == "listVolumes" {
			c.Assert(r.URL.Query().Get("virtualmachineid"), gocheck.Equals, "myMachineId")
			fmt.Fprintln(w, `{"listvolumesresponse": {"volume": [ {"id": "v1", "type": "ROOT"}, {"id": "v2", "type": "DATADISK"} ]}}`)
		}
		if cmd == "destroyVirtualMachine" {
			c.Assert(r.URL.Query().Get("id"), gocheck.Equals, "myMachineId")
			fmt.Fprintln(w, `{"destroyvirtualmachineresponse": {"jobid": "job1"}}`)
		}
		if cmd == "queryAsyncJobResult" {
			c.Assert(r.URL.Query().Get("jobid"), gocheck.Equals, "job1")
			fmt.Fprintln(w, `{"queryasyncjobresultresponse": {"jobstatus": 1}}`)
		}
		if cmd == "detachVolume" {
			c.Assert(r.URL.Query().Get("id"), gocheck.Equals, "v2")
			fmt.Fprintln(w, `{"detachvolumeresponse": {"jobid": "job1"}}`)
		}
		if cmd == "deleteVolume" {
			c.Assert(r.URL.Query().Get("id"), gocheck.Equals, "v2")
		}
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	cs := NewCloudstackIaaS()
	machine := iaas.Machine{Id: "myMachineId", CreationParams: map[string]string{"projectid": "projid"}}
	err := cs.DeleteMachine(&machine)
	c.Assert(err, gocheck.IsNil)
	c.Assert(calls, gocheck.DeepEquals, []string{
		"listVolumes",
		"destroyVirtualMachine",
		"queryAsyncJobResult",
		"detachVolume",
		"queryAsyncJobResult",
		"deleteVolume",
	})
}

func (s *cloudstackSuite) TestDeleteMachineAsyncFail(c *gocheck.C) {
	var calls []string
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cmd := r.URL.Query().Get("command")
		calls = append(calls, cmd)
		w.Header().Set("Content-type", "application/json")
		if cmd == "listVolumes" {
			c.Assert(r.URL.Query().Get("virtualmachineid"), gocheck.Equals, "myMachineId")
			fmt.Fprintln(w, `{"listvolumesresponse": {"volume": [  ]}}`)
		}
		if cmd == "destroyVirtualMachine" {
			c.Assert(r.URL.Query().Get("id"), gocheck.Equals, "myMachineId")
			fmt.Fprintln(w, `{"destroyvirtualmachineresponse": {"jobid": "job1"}}`)
		}
		if cmd == "queryAsyncJobResult" {
			c.Assert(r.URL.Query().Get("jobid"), gocheck.Equals, "job1")
			fmt.Fprintln(w, `{"queryasyncjobresultresponse": {"jobstatus": 2, "jobresult": "my awesome err"}}`)
		}
	}))
	defer fakeServer.Close()
	config.Set("iaas:cloudstack:url", fakeServer.URL)
	cs := NewCloudstackIaaS()
	machine := iaas.Machine{Id: "myMachineId", CreationParams: map[string]string{"projectid": "projid"}}
	err := cs.DeleteMachine(&machine)
	c.Assert(err, gocheck.ErrorMatches, ".*my awesome err.*")
	c.Assert(calls, gocheck.DeepEquals, []string{
		"listVolumes",
		"destroyVirtualMachine",
		"queryAsyncJobResult",
	})
}

func (s *cloudstackSuite) TestDeleteMachineError(c *gocheck.C) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	config.Set("iaas:cloudstack:url", server.URL)
	defer server.Close()
	cs := NewCloudstackIaaS()
	machine := iaas.Machine{Id: "myMachineId"}
	err := cs.DeleteMachine(&machine)
	c.Assert(err, gocheck.ErrorMatches, ".*Unexpected response code.*")
}

func (s *cloudstackSuite) TestDeleteMachineErrorNoServer(c *gocheck.C) {
	config.Set("iaas:cloudstack:url", "http://invalidurl.invalid.invalid")
	cs := NewCloudstackIaaS()
	machine := iaas.Machine{Id: "myMachineId"}
	err := cs.DeleteMachine(&machine)
	c.Assert(err, gocheck.ErrorMatches, ".*no such host.*")
}

func (s *cloudstackSuite) TestClone(c *gocheck.C) {
	cs := NewCloudstackIaaS()
	clonned := cs.Clone("something")
	c.Assert(clonned, gocheck.FitsTypeOf, cs)
	clonnedCS, _ := clonned.(*CloudstackIaaS)
	c.Assert(cs.base.IaaSName, gocheck.Equals, "")
	c.Assert(clonnedCS.base.IaaSName, gocheck.Equals, "something")
}
