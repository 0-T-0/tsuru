// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/tsuru/config"
	"github.com/tsuru/docker-cluster/cluster"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/testing"
	"io/ioutil"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
	"net/http"
	"net/http/httptest"
	"strings"
)

type HandlersSuite struct {
	conn *db.Storage
}

var _ = gocheck.Suite(&HandlersSuite{})

func (s *HandlersSuite) SetUpSuite(c *gocheck.C) {
	var err error
	s.conn, err = db.Conn()
	c.Assert(err, gocheck.IsNil)
	config.Set("docker:collection", "docker_handler_suite")
	s.conn.Collection(schedulerCollection).RemoveAll(nil)
}

func (s *HandlersSuite) TearDownSuite(c *gocheck.C) {
	coll := collection()
	defer coll.Close()
	err := coll.Database.DropDatabase()
	c.Assert(err, gocheck.IsNil)
	s.conn.Close()
}

func (s *HandlersSuite) TestAddNodeHandler(c *gocheck.C) {
	dCluster, _ = cluster.New(segScheduler, nil)
	b := bytes.NewBufferString(`{"address": "host.com:4243", "ID": "server01", "teams": "myteam"}`)
	req, err := http.NewRequest("POST", "/node/add", b)
	c.Assert(err, gocheck.IsNil)
	rec := httptest.NewRecorder()
	err = addNodeHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server01")
	n, err := s.conn.Collection(schedulerCollection).FindId("server01").Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 1)
}

func (s *HandlersSuite) TestAddNodeHandlerWithoutdCluster(c *gocheck.C) {
	config.Set("docker:segregate", true)
	defer config.Unset("docker:segregate")
	config.Set("docker:scheduler:redis-server", "127.0.0.1:6379")
	defer config.Unset("docker:scheduler:redis-server")
	dCluster = nil
	b := bytes.NewBufferString(`{"address": "host.com:4243", "ID": "server01", "teams": "myteam"}`)
	req, err := http.NewRequest("POST", "/node/add", b)
	c.Assert(err, gocheck.IsNil)
	rec := httptest.NewRecorder()
	err = addNodeHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server01")
	n, err := s.conn.Collection(schedulerCollection).FindId("server01").Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 1)
}

func (s *HandlersSuite) TestRemoveNodeHandler(c *gocheck.C) {
	dCluster, _ = cluster.New(segScheduler, nil)
	err := s.conn.Collection(schedulerCollection).Insert(map[string]string{"address": "host.com:4243", "_id": "server01", "teams": "myteam"})
	c.Assert(err, gocheck.IsNil)
	n, err := s.conn.Collection(schedulerCollection).FindId("server01").Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 1)
	b := bytes.NewBufferString(`{"ID": "server01"}`)
	req, err := http.NewRequest("POST", "/node/remove", b)
	c.Assert(err, gocheck.IsNil)
	rec := httptest.NewRecorder()
	err = removeNodeHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server01")
	n, err = s.conn.Collection(schedulerCollection).FindId("server01").Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 0)
}

func (s *HandlersSuite) TestRemoveNodeHandlerWithoutCluster(c *gocheck.C) {
	config.Set("docker:segregate", true)
	defer config.Unset("docker:segregate")
	config.Set("docker:scheduler:redis-server", "127.0.0.1:6379")
	defer config.Unset("docker:scheduler:redis-server")
	dCluster = nil
	err := s.conn.Collection(schedulerCollection).Insert(map[string]string{"address": "host.com:4243", "_id": "server01", "teams": "myteam"})
	c.Assert(err, gocheck.IsNil)
	n, err := s.conn.Collection(schedulerCollection).FindId("server01").Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 1)
	b := bytes.NewBufferString(`{"ID": "server01"}`)
	req, err := http.NewRequest("POST", "/node/remove", b)
	c.Assert(err, gocheck.IsNil)
	rec := httptest.NewRecorder()
	err = removeNodeHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server01")
	n, err = s.conn.Collection(schedulerCollection).FindId("server01").Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 0)
}

func (s *HandlersSuite) TestListNodeHandler(c *gocheck.C) {
	var result []node
	dCluster, _ = cluster.New(segScheduler, nil)
	err := s.conn.Collection(schedulerCollection).Insert(node{Address: "host.com:4243", ID: "server01"})
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server01")
	err = s.conn.Collection(schedulerCollection).Insert(node{Address: "host.com:4243", ID: "server02"})
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server02")
	req, err := http.NewRequest("GET", "/node/", nil)
	rec := httptest.NewRecorder()
	err = listNodeHandler(rec, req, nil)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	err = json.Unmarshal(body, &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(result[0].ID, gocheck.Equals, "server01")
	c.Assert(result[0].Address, gocheck.DeepEquals, "host.com:4243")
	c.Assert(result[1].ID, gocheck.Equals, "server02")
	c.Assert(result[1].Address, gocheck.DeepEquals, "host.com:4243")
}

func (s *HandlersSuite) TestListNodeHandlerWithoutCluster(c *gocheck.C) {
	var result []node
	config.Set("docker:segregate", true)
	defer config.Unset("docker:segregate")
	config.Set("docker:scheduler:redis-server", "127.0.0.1:6379")
	defer config.Unset("docker:scheduler:redis-server")
	dCluster = nil
	err := s.conn.Collection(schedulerCollection).Insert(node{Address: "host.com:4243", ID: "server01"})
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server01")
	err = s.conn.Collection(schedulerCollection).Insert(node{Address: "host.com:4243", ID: "server02"})
	c.Assert(err, gocheck.IsNil)
	defer s.conn.Collection(schedulerCollection).RemoveId("server02")
	req, err := http.NewRequest("GET", "/node/", nil)
	rec := httptest.NewRecorder()
	err = listNodeHandler(rec, req, nil)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	err = json.Unmarshal(body, &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(result[0].ID, gocheck.Equals, "server01")
	c.Assert(result[0].Address, gocheck.DeepEquals, "host.com:4243")
	c.Assert(result[1].ID, gocheck.Equals, "server02")
	c.Assert(result[1].Address, gocheck.DeepEquals, "host.com:4243")
}

func (s *HandlersSuite) TestListContainersByHostHandler(c *gocheck.C) {
	var result []container
	coll := collection()
	dCluster, _ = cluster.New(segScheduler, nil)
	err := coll.Insert(container{ID: "blabla", Type: "python", HostAddr: "http://cittavld1182.globoi.com"})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"id": "blabla"})
	err = coll.Insert(container{ID: "bleble", Type: "java", HostAddr: "http://cittavld1182.globoi.com"})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"id": "bleble"})
	req, err := http.NewRequest("GET", "/node/cittavld1182.globoi.com/containers?:address=http://cittavld1182.globoi.com", nil)
	rec := httptest.NewRecorder()
	err = listContainersHandler(rec, req, nil)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	err = json.Unmarshal(body, &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(result[0].ID, gocheck.DeepEquals, "blabla")
	c.Assert(result[0].Type, gocheck.DeepEquals, "python")
	c.Assert(result[0].HostAddr, gocheck.DeepEquals, "http://cittavld1182.globoi.com")
	c.Assert(result[1].ID, gocheck.DeepEquals, "bleble")
	c.Assert(result[1].Type, gocheck.DeepEquals, "java")
	c.Assert(result[1].HostAddr, gocheck.DeepEquals, "http://cittavld1182.globoi.com")
}

func (s *HandlersSuite) TestListContainersByAppHandler(c *gocheck.C) {
	var result []container
	coll := collection()
	dCluster, _ = cluster.New(segScheduler, nil)
	err := coll.Insert(container{ID: "blabla", AppName: "appbla", HostAddr: "http://cittavld1182.globoi.com"})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"id": "blabla"})
	err = coll.Insert(container{ID: "bleble", AppName: "appbla", HostAddr: "http://cittavld1180.globoi.com"})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"id": "bleble"})
	req, err := http.NewRequest("GET", "/node/appbla/containers?:appname=appbla", nil)
	rec := httptest.NewRecorder()
	err = listContainersHandler(rec, req, nil)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	err = json.Unmarshal(body, &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(result[0].ID, gocheck.DeepEquals, "blabla")
	c.Assert(result[0].AppName, gocheck.DeepEquals, "appbla")
	c.Assert(result[0].HostAddr, gocheck.DeepEquals, "http://cittavld1182.globoi.com")
	c.Assert(result[1].ID, gocheck.DeepEquals, "bleble")
	c.Assert(result[1].AppName, gocheck.DeepEquals, "appbla")
	c.Assert(result[1].HostAddr, gocheck.DeepEquals, "http://cittavld1180.globoi.com")
}

func (s *HandlersSuite) TestMoveContainersEmptyBodyHandler(c *gocheck.C) {
	b := bytes.NewBufferString("")
	req, err := http.NewRequest("POST", "/containers/move", b)
	rec := httptest.NewRecorder()
	err = moveContainersHandler(rec, req)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "unexpected end of JSON input")
}

func (s *HandlersSuite) TestMoveContainersEmptyToHandler(c *gocheck.C) {
	b := bytes.NewBufferString(`{"from": "fromhost", "to": ""}`)
	req, err := http.NewRequest("POST", "/containers/move", b)
	rec := httptest.NewRecorder()
	err = moveContainersHandler(rec, req)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Invalid params: from: fromhost - to: ")
}

func (s *HandlersSuite) TestMoveContainersHandler(c *gocheck.C) {
	b := bytes.NewBufferString(`{"from": "localhost", "to": "127.0.0.1"}`)
	req, err := http.NewRequest("POST", "/containers/move", b)
	rec := httptest.NewRecorder()
	err = moveContainersHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	validJson := fmt.Sprintf("[%s]", strings.Replace(strings.Trim(string(body), "\n "), "\n", ",", -1))
	var result []progressLog
	err = json.Unmarshal([]byte(validJson), &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(result, gocheck.DeepEquals, []progressLog{
		{Message: "No units to move in localhost."},
		{Message: "Containers moved successfully!"},
	})
}

func (s *S) TestRebalanceContainersEmptyBodyHandler(c *gocheck.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, gocheck.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	appInstance := testing.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	provisionMutex.Lock()
	oldProvisioner := app.Provisioner
	app.Provisioner = &p
	provisionMutex.Unlock()
	defer func() {
		provisionMutex.Lock()
		app.Provisioner = oldProvisioner
		provisionMutex.Unlock()
	}()
	coll := collection()
	defer coll.Close()
	coll.Insert(container{ID: "container-id", AppName: appInstance.GetName(), Version: "container-version", Image: "tsuru/python"})
	defer coll.RemoveAll(bson.M{"appname": appInstance.GetName()})
	units, err := addUnitsWithHost(appInstance, 5, "localhost")
	c.Assert(err, gocheck.IsNil)
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name:     appInstance.GetName(),
		Platform: appInstance.GetPlatform(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	err = conn.Apps().Update(
		bson.M{"name": appStruct.Name},
		bson.M{"$set": bson.M{"units": units}},
	)
	c.Assert(err, gocheck.IsNil)
	b := bytes.NewBufferString("")
	req, err := http.NewRequest("POST", "/containers/move", b)
	rec := httptest.NewRecorder()
	err = rebalanceContainersHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	validJson := fmt.Sprintf("[%s]", strings.Replace(strings.Trim(string(body), "\n "), "\n", ",", -1))
	var result []progressLog
	err = json.Unmarshal([]byte(validJson), &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(len(result), gocheck.Equals, 8)
	c.Assert(result[0].Message, gocheck.Equals, "Trying to move 2 units from localhost...")
	c.Assert(result[1].Message, gocheck.Matches, "Moving unit .*")
	c.Assert(result[2].Message, gocheck.Matches, "Moving unit .*")
	c.Assert(result[7].Message, gocheck.Matches, "Containers rebalanced successfully!")
	testing.CleanQ("tsuru-app")
}

func (s *S) TestRebalanceContainersDryBodyHandler(c *gocheck.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, gocheck.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/python", s.server.URL())
	c.Assert(err, gocheck.IsNil)
	appInstance := testing.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	provisionMutex.Lock()
	oldProvisioner := app.Provisioner
	app.Provisioner = &p
	provisionMutex.Unlock()
	defer func() {
		provisionMutex.Lock()
		app.Provisioner = oldProvisioner
		provisionMutex.Unlock()
	}()
	coll := collection()
	defer coll.Close()
	coll.Insert(container{ID: "container-id", AppName: appInstance.GetName(), Version: "container-version", Image: "tsuru/python"})
	defer coll.RemoveAll(bson.M{"appname": appInstance.GetName()})
	units, err := addUnitsWithHost(appInstance, 5, "localhost")
	c.Assert(err, gocheck.IsNil)
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name:     appInstance.GetName(),
		Platform: appInstance.GetPlatform(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	err = conn.Apps().Update(
		bson.M{"name": appStruct.Name},
		bson.M{"$set": bson.M{"units": units}},
	)
	c.Assert(err, gocheck.IsNil)
	b := bytes.NewBufferString(`{"dry": "true"}`)
	req, err := http.NewRequest("POST", "/containers/move", b)
	rec := httptest.NewRecorder()
	err = rebalanceContainersHandler(rec, req)
	c.Assert(err, gocheck.IsNil)
	body, err := ioutil.ReadAll(rec.Body)
	c.Assert(err, gocheck.IsNil)
	validJson := fmt.Sprintf("[%s]", strings.Replace(strings.Trim(string(body), "\n "), "\n", ",", -1))
	var result []progressLog
	err = json.Unmarshal([]byte(validJson), &result)
	c.Assert(err, gocheck.IsNil)
	c.Assert(len(result), gocheck.Equals, 4)
	c.Assert(result[0].Message, gocheck.Equals, "Trying to move 2 units from localhost...")
	c.Assert(result[1].Message, gocheck.Matches, "Would move unit .*")
	c.Assert(result[2].Message, gocheck.Matches, "Would move unit .*")
	c.Assert(result[3].Message, gocheck.Matches, "Containers rebalanced successfully!")
	testing.CleanQ("tsuru-app")
}
