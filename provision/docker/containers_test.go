// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/provision/provisiontest"
	"gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

func (s *S) TestMoveContainers(c *check.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/app-myapp", s.server.URL())
	c.Assert(err, check.IsNil)
	appInstance := provisiontest.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	coll := collection()
	defer coll.Close()
	coll.Insert(container{ID: "container-id", AppName: appInstance.GetName(), Version: "container-version", Image: "tsuru/python"})
	defer coll.RemoveAll(bson.M{"appname": appInstance.GetName()})
	imageId, err := appCurrentImageName(appInstance.GetName())
	c.Assert(err, check.IsNil)
	_, err = addContainersWithHost(&changeUnitsPipelineArgs{
		toHost:     "localhost",
		unitsToAdd: 2,
		app:        appInstance,
		imageId:    imageId,
	})
	c.Assert(err, check.IsNil)
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, check.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = moveContainers("localhost", "127.0.0.1", encoder)
	c.Assert(err, check.IsNil)
	containers, err := listContainersByHost("localhost")
	c.Assert(len(containers), check.Equals, 0)
	containers, err = listContainersByHost("127.0.0.1")
	c.Assert(len(containers), check.Equals, 2)
	parts := strings.Split(buf.String(), "\n")
	var logEntry progressLog
	json.Unmarshal([]byte(parts[0]), &logEntry)
	c.Assert(logEntry.Message, check.Matches, ".*Moving 2 units.*")
	json.Unmarshal([]byte(parts[1]), &logEntry)
	c.Assert(logEntry.Message, check.Matches, ".*Moving unit.*for.*myapp.*localhost.*127.0.0.1.*")
	json.Unmarshal([]byte(parts[2]), &logEntry)
	c.Assert(logEntry.Message, check.Matches, ".*Moving unit.*for.*myapp.*localhost.*127.0.0.1.*")
}

func (s *S) TestMoveContainersUnknownDest(c *check.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/app-myapp", s.server.URL())
	c.Assert(err, check.IsNil)
	appInstance := provisiontest.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	coll := collection()
	defer coll.Close()
	coll.Insert(container{ID: "container-id", AppName: appInstance.GetName(), Version: "container-version", Image: "tsuru/python"})
	defer coll.RemoveAll(bson.M{"appname": appInstance.GetName()})
	imageId, err := appCurrentImageName(appInstance.GetName())
	c.Assert(err, check.IsNil)
	_, err = addContainersWithHost(&changeUnitsPipelineArgs{
		toHost:     "localhost",
		unitsToAdd: 2,
		app:        appInstance,
		imageId:    imageId,
	})
	c.Assert(err, check.IsNil)
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, check.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = moveContainers("localhost", "unknown", encoder)
	c.Assert(err, check.Equals, containerMovementErr)
	parts := strings.Split(buf.String(), "\n")
	var logEntry progressLog
	json.Unmarshal([]byte(parts[0]), &logEntry)
	c.Assert(logEntry.Message, check.Matches, ".*Moving 2 units.*")
	json.Unmarshal([]byte(parts[3]), &logEntry)
	c.Assert(logEntry.Message, check.Matches, ".*Error moving unit.*Caused by:.*unknown.*not found")
	json.Unmarshal([]byte(parts[4]), &logEntry)
	c.Assert(logEntry.Message, check.Matches, ".*Error moving unit.*Caused by:.*unknown.*not found")
}

func (s *S) TestMoveContainer(c *check.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/app-myapp", s.server.URL())
	c.Assert(err, check.IsNil)
	appInstance := provisiontest.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	coll := collection()
	defer coll.Close()
	coll.Insert(container{ID: "container-id", AppName: appInstance.GetName(), Version: "container-version", Image: "tsuru/python"})
	defer coll.RemoveAll(bson.M{"appname": appInstance.GetName()})
	imageId, err := appCurrentImageName(appInstance.GetName())
	c.Assert(err, check.IsNil)
	addedConts, err := addContainersWithHost(&changeUnitsPipelineArgs{
		toHost:     "localhost",
		unitsToAdd: 2,
		app:        appInstance,
		imageId:    imageId,
	})
	c.Assert(err, check.IsNil)
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, check.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	_, err = moveContainer(addedConts[0].ID[0:6], "127.0.0.1", encoder)
	c.Assert(err, check.IsNil)
	containers, err := listContainersByHost("localhost")
	c.Assert(len(containers), check.Equals, 1)
	containers, err = listContainersByHost("127.0.0.1")
	c.Assert(len(containers), check.Equals, 1)
}

func (s *S) TestRebalanceContainers(c *check.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/app-myapp", s.server.URL())
	c.Assert(err, check.IsNil)
	appInstance := provisiontest.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	imageId, err := appCurrentImageName(appInstance.GetName())
	c.Assert(err, check.IsNil)
	_, err = addContainersWithHost(&changeUnitsPipelineArgs{
		toHost:     "localhost",
		unitsToAdd: 5,
		app:        appInstance,
		imageId:    imageId,
	})
	c.Assert(err, check.IsNil)
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, check.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err = rebalanceContainers(encoder, false)
	c.Assert(err, check.IsNil)
	c1, err := listContainersByHost("localhost")
	c.Assert(err, check.IsNil)
	c2, err := listContainersByHost("127.0.0.1")
	c.Assert(err, check.IsNil)
	c.Assert((len(c1) == 3 && len(c2) == 2) || (len(c1) == 2 && len(c2) == 3), check.Equals, true)
}

func (s *S) TestAppLocker(c *check.C) {
	appName := "myapp"
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appDB := &app.App{Name: appName}
	defer conn.Apps().Remove(bson.M{"name": appName})
	err = conn.Apps().Insert(appDB)
	c.Assert(err, check.IsNil)
	locker := &appLocker{}
	hasLock := locker.lock(appName)
	c.Assert(hasLock, check.Equals, true)
	c.Assert(locker.refCount[appName], check.Equals, 1)
	appDB, err = app.GetByName(appName)
	c.Assert(err, check.IsNil)
	c.Assert(appDB.Lock.Locked, check.Equals, true)
	c.Assert(appDB.Lock.Owner, check.Equals, app.InternalAppName)
	c.Assert(appDB.Lock.Reason, check.Equals, "container-move")
	hasLock = locker.lock(appName)
	c.Assert(hasLock, check.Equals, true)
	c.Assert(locker.refCount[appName], check.Equals, 2)
	locker.unlock(appName)
	c.Assert(locker.refCount[appName], check.Equals, 1)
	appDB, err = app.GetByName(appName)
	c.Assert(err, check.IsNil)
	c.Assert(appDB.Lock.Locked, check.Equals, true)
	locker.unlock(appName)
	c.Assert(locker.refCount[appName], check.Equals, 0)
	appDB, err = app.GetByName(appName)
	c.Assert(err, check.IsNil)
	c.Assert(appDB.Lock.Locked, check.Equals, false)
}

func (s *S) TestAppLockerBlockOtherLockers(c *check.C) {
	appName := "myapp"
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appDB := &app.App{Name: appName}
	defer conn.Apps().Remove(bson.M{"name": appName})
	err = conn.Apps().Insert(appDB)
	c.Assert(err, check.IsNil)
	locker := &appLocker{}
	hasLock := locker.lock(appName)
	c.Assert(hasLock, check.Equals, true)
	c.Assert(locker.refCount[appName], check.Equals, 1)
	appDB, err = app.GetByName(appName)
	c.Assert(err, check.IsNil)
	c.Assert(appDB.Lock.Locked, check.Equals, true)
	otherLocker := &appLocker{}
	hasLock = otherLocker.lock(appName)
	c.Assert(hasLock, check.Equals, false)
}

func (s *S) TestRebalanceContainersManyApps(c *check.C) {
	c.ExpectFailure("Rebalance scheduling does not distribute applications across hosts")
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	err = newImage("tsuru/app-myapp", s.server.URL())
	c.Assert(err, check.IsNil)
	err = newImage("tsuru/app-otherapp", s.server.URL())
	c.Assert(err, check.IsNil)
	appInstance := provisiontest.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	appInstance2 := provisiontest.NewFakeApp("otherapp", "python", 0)
	defer p.Destroy(appInstance2)
	p.Provision(appInstance2)
	imageId, err := appCurrentImageName(appInstance.GetName())
	c.Assert(err, check.IsNil)
	_, err = addContainersWithHost(&changeUnitsPipelineArgs{
		toHost:     "localhost",
		unitsToAdd: 1,
		app:        appInstance,
		imageId:    imageId,
	})
	c.Assert(err, check.IsNil)
	imageId2, err := appCurrentImageName(appInstance2.GetName())
	c.Assert(err, check.IsNil)
	_, err = addContainersWithHost(&changeUnitsPipelineArgs{
		toHost:     "localhost",
		unitsToAdd: 1,
		app:        appInstance2,
		imageId:    imageId2,
	})
	c.Assert(err, check.IsNil)
	conn, err := db.Conn()
	c.Assert(err, check.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, check.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})
	appStruct2 := &app.App{
		Name: appInstance2.GetName(),
	}
	err = conn.Apps().Insert(appStruct2)
	c.Assert(err, check.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct2.Name})
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	c1, err := listContainersByHost("localhost")
	c.Assert(len(c1), check.Equals, 2)
	err = rebalanceContainers(encoder, false)
	c.Assert(err, check.IsNil)
	c1, err = listContainersByHost("localhost")
	c.Assert(len(c1), check.Equals, 1)
	c2, err := listContainersByHost("127.0.0.1")
	c.Assert(len(c2), check.Equals, 1)
}
