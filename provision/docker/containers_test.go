// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/testing"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
	"strings"
	"sync"
)

var provisionMutex sync.Mutex

func (s *S) TestMoveContainers(c *gocheck.C) {
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
	units, err := addUnitsWithHost(appInstance, 2, "localhost")
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
	err = conn.Apps().Update(
		bson.M{"name": appStruct.Name},
		bson.M{"$set": bson.M{"units": units}},
	)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = moveContainers("localhost", "127.0.0.1", encoder)
	c.Assert(err, gocheck.IsNil)
	containers, err := listContainersByHost("localhost")
	c.Assert(len(containers), gocheck.Equals, 0)
	containers, err = listContainersByHost("127.0.0.1")
	c.Assert(len(containers), gocheck.Equals, 2)
	parts := strings.Split(buf.String(), "\n")
	var logEntry progressLog
	json.Unmarshal([]byte(parts[0]), &logEntry)
	c.Assert(logEntry.Message, gocheck.Matches, ".*Moving 2 units.*")
	json.Unmarshal([]byte(parts[1]), &logEntry)
	c.Assert(logEntry.Message, gocheck.Matches, ".*Moving unit.*for.*myapp.*localhost.*127.0.0.1.*")
	json.Unmarshal([]byte(parts[2]), &logEntry)
	c.Assert(logEntry.Message, gocheck.Matches, ".*Moving unit.*for.*myapp.*localhost.*127.0.0.1.*")
	testing.CleanQ("tsuru-app")
}

func (s *S) TestMoveContainersUnknownDest(c *gocheck.C) {
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
	units, err := addUnitsWithHost(appInstance, 2, "localhost")
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
	err = conn.Apps().Update(
		bson.M{"name": appStruct.Name},
		bson.M{"$set": bson.M{"units": units}},
	)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = moveContainers("localhost", "unknown", encoder)
	c.Assert(err, gocheck.Equals, containerMovementErr)
	parts := strings.Split(buf.String(), "\n")
	var logEntry progressLog
	json.Unmarshal([]byte(parts[0]), &logEntry)
	c.Assert(logEntry.Message, gocheck.Matches, ".*Moving 2 units.*")
	json.Unmarshal([]byte(parts[3]), &logEntry)
	c.Assert(logEntry.Message, gocheck.Matches, ".*Error moving unit.*Caused by:.*unknown.*not found")
	json.Unmarshal([]byte(parts[4]), &logEntry)
	c.Assert(logEntry.Message, gocheck.Matches, ".*Error moving unit.*Caused by:.*unknown.*not found")
}

func (s *S) TestMoveContainer(c *gocheck.C) {
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
	units, err := addUnitsWithHost(appInstance, 2, "localhost")
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
	err = conn.Apps().Update(
		bson.M{"name": appStruct.Name},
		bson.M{"$set": bson.M{"units": units}},
	)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = moveContainer(units[0].Name[0:6], "127.0.0.1", encoder)
	c.Assert(err, gocheck.IsNil)
	containers, err := listContainersByHost("localhost")
	c.Assert(len(containers), gocheck.Equals, 1)
	containers, err = listContainersByHost("127.0.0.1")
	c.Assert(len(containers), gocheck.Equals, 1)
	testing.CleanQ("tsuru-app")
}

func (s *S) TestRebalanceContainers(c *gocheck.C) {
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
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = rebalanceContainers(encoder, false)
	c.Assert(err, gocheck.IsNil)
	c1, err := listContainersByHost("localhost")
	c.Assert(len(c1), gocheck.Equals, 3)
	c2, err := listContainersByHost("127.0.0.1")
	c.Assert(len(c2), gocheck.Equals, 2)
	testing.CleanQ("tsuru-app")
}
