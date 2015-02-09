// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"reflect"
	"sort"

	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/tsuru/action"
	"github.com/tsuru/tsuru/provision/provisiontest"
	"github.com/tsuru/tsuru/router/routertest"
	"gopkg.in/check.v1"
	"gopkg.in/mgo.v2/bson"
)

func (s *S) TestInsertEmptyContainerInDBName(c *check.C) {
	c.Assert(insertEmptyContainerInDB.Name, check.Equals, "insert-empty-container")
}

func (s *S) TestInsertEmptyContainerInDBForward(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	args := runContainerActionsArgs{app: app, imageID: "image-id", buildingImage: "next-image"}
	context := action.FWContext{Params: []interface{}{args}}
	r, err := insertEmptyContainerInDB.Forward(context)
	c.Assert(err, check.IsNil)
	cont := r.(container)
	c.Assert(cont, check.FitsTypeOf, container{})
	c.Assert(cont.AppName, check.Equals, app.GetName())
	c.Assert(cont.Type, check.Equals, app.GetPlatform())
	c.Assert(cont.Name, check.Not(check.Equals), "")
	c.Assert(cont.Name, check.HasLen, 20)
	c.Assert(cont.Status, check.Equals, "created")
	c.Assert(cont.Image, check.Equals, "image-id")
	c.Assert(cont.BuildingImage, check.Equals, "next-image")
	coll := collection()
	defer coll.Close()
	defer coll.Remove(bson.M{"name": cont.Name})
	var retrieved container
	err = coll.Find(bson.M{"name": cont.Name}).One(&retrieved)
	c.Assert(err, check.IsNil)
	c.Assert(retrieved.Name, check.Equals, cont.Name)
}

func (s *S) TestInsertEmptyContainerInDBBackward(c *check.C) {
	cont := container{Name: "myName"}
	coll := collection()
	defer coll.Close()
	err := coll.Insert(&cont)
	c.Assert(err, check.IsNil)
	context := action.BWContext{FWResult: cont}
	insertEmptyContainerInDB.Backward(context)
	err = coll.Find(bson.M{"name": cont.Name}).One(&cont)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "not found")
}

func (s *S) TestUpdateContainerInDBName(c *check.C) {
	c.Assert(updateContainerInDB.Name, check.Equals, "update-database-container")
}

func (s *S) TestUpdateContainerInDBForward(c *check.C) {
	cont := container{Name: "myName"}
	coll := collection()
	defer coll.Close()
	err := coll.Insert(cont)
	c.Assert(err, check.IsNil)
	cont.ID = "myID"
	context := action.FWContext{Previous: cont}
	r, err := updateContainerInDB.Forward(context)
	c.Assert(r, check.FitsTypeOf, container{})
	retrieved, err := getContainer(cont.ID)
	c.Assert(err, check.IsNil)
	c.Assert(retrieved.ID, check.Equals, cont.ID)
}

func (s *S) TestCreateContainerName(c *check.C) {
	c.Assert(createContainer.Name, check.Equals, "create-container")
}

func (s *S) TestCreateContainerForward(c *check.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	client, err := docker.NewClient(s.server.URL())
	c.Assert(err, check.IsNil)
	images, err := client.ListImages(docker.ListImagesOptions{All: true})
	c.Assert(err, check.IsNil)
	cmds := []string{"ps", "-ef"}
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	cont := container{Name: "myName", AppName: app.GetName(), Type: app.GetPlatform(), Status: "created"}
	args := runContainerActionsArgs{app: app, imageID: images[0].ID, commands: cmds}
	context := action.FWContext{Previous: cont, Params: []interface{}{args}}
	r, err := createContainer.Forward(context)
	c.Assert(err, check.IsNil)
	cont = r.(container)
	defer cont.remove()
	c.Assert(cont, check.FitsTypeOf, container{})
	c.Assert(cont.ID, check.Not(check.Equals), "")
	c.Assert(cont.HostAddr, check.Equals, "127.0.0.1")
	dcli, err := docker.NewClient(s.server.URL())
	c.Assert(err, check.IsNil)
	cc, err := dcli.InspectContainer(cont.ID)
	c.Assert(err, check.IsNil)
	c.Assert(cc.State.Running, check.Equals, false)
}

func (s *S) TestCreateContainerBackward(c *check.C) {
	dcli, err := docker.NewClient(s.server.URL())
	c.Assert(err, check.IsNil)
	err = newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	defer dcli.RemoveImage("tsuru/python")
	conta, err := s.newContainer(nil)
	c.Assert(err, check.IsNil)
	defer s.removeTestContainer(conta)
	cont := *conta
	context := action.BWContext{FWResult: cont}
	createContainer.Backward(context)
	_, err = dcli.InspectContainer(cont.ID)
	c.Assert(err, check.NotNil)
	c.Assert(err, check.FitsTypeOf, &docker.NoSuchContainer{})
}

func (s *S) TestAddNewRouteName(c *check.C) {
	c.Assert(addNewRoutes.Name, check.Equals, "add-new-routes")
}

func (s *S) TestAddNewRouteForward(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	routertest.FakeRouter.AddBackend(app.GetName())
	defer routertest.FakeRouter.RemoveBackend(app.GetName())
	cont := container{ID: "ble-1", AppName: app.GetName()}
	cont2 := container{ID: "ble-2", AppName: app.GetName()}
	defer cont.remove()
	defer cont2.remove()
	args := changeUnitsPipelineArgs{
		app: app,
	}
	context := action.FWContext{Previous: []container{cont, cont2}, Params: []interface{}{args}}
	r, err := addNewRoutes.Forward(context)
	c.Assert(err, check.IsNil)
	containers := r.([]container)
	hasRoute := routertest.FakeRouter.HasRoute(app.GetName(), cont.getAddress())
	c.Assert(hasRoute, check.Equals, true)
	hasRoute = routertest.FakeRouter.HasRoute(app.GetName(), cont2.getAddress())
	c.Assert(hasRoute, check.Equals, true)
	c.Assert(containers, check.DeepEquals, []container{cont, cont2})
}

func (s *S) TestAddNewRouteForwardFailInMiddle(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	routertest.FakeRouter.AddBackend(app.GetName())
	defer routertest.FakeRouter.RemoveBackend(app.GetName())
	cont := container{ID: "ble-1", AppName: app.GetName()}
	cont2 := container{ID: "ble-2", AppName: app.GetName()}
	defer cont.remove()
	defer cont2.remove()
	routertest.FakeRouter.FailForIp(cont2.getAddress())
	args := changeUnitsPipelineArgs{
		app: app,
	}
	context := action.FWContext{Previous: []container{cont, cont2}, Params: []interface{}{args}}
	_, err := addNewRoutes.Forward(context)
	c.Assert(err, check.Equals, routertest.ErrForcedFailure)
	hasRoute := routertest.FakeRouter.HasRoute(app.GetName(), cont.getAddress())
	c.Assert(hasRoute, check.Equals, false)
	hasRoute = routertest.FakeRouter.HasRoute(app.GetName(), cont2.getAddress())
	c.Assert(hasRoute, check.Equals, false)
}

func (s *S) TestAddNewRouteBackward(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	routertest.FakeRouter.AddBackend(app.GetName())
	defer routertest.FakeRouter.RemoveBackend(app.GetName())
	cont := container{ID: "ble-1", AppName: app.GetName()}
	cont2 := container{ID: "ble-2", AppName: app.GetName()}
	defer cont.remove()
	defer cont2.remove()
	err := routertest.FakeRouter.AddRoute(app.GetName(), cont.getAddress())
	c.Assert(err, check.IsNil)
	err = routertest.FakeRouter.AddRoute(app.GetName(), cont2.getAddress())
	c.Assert(err, check.IsNil)
	args := changeUnitsPipelineArgs{
		app: app,
	}
	context := action.BWContext{FWResult: []container{cont, cont2}, Params: []interface{}{args}}
	addNewRoutes.Backward(context)
	hasRoute := routertest.FakeRouter.HasRoute(app.GetName(), cont.getAddress())
	c.Assert(hasRoute, check.Equals, false)
	hasRoute = routertest.FakeRouter.HasRoute(app.GetName(), cont2.getAddress())
	c.Assert(hasRoute, check.Equals, false)
}

func (s *S) TestRemoveOldRoutesName(c *check.C) {
	c.Assert(removeOldRoutes.Name, check.Equals, "remove-old-routes")
}

func (s *S) TestRemoveOldRoutesForward(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	routertest.FakeRouter.AddBackend(app.GetName())
	defer routertest.FakeRouter.RemoveBackend(app.GetName())
	cont := container{ID: "ble-1", AppName: app.GetName()}
	cont2 := container{ID: "ble-2", AppName: app.GetName()}
	defer cont.remove()
	defer cont2.remove()
	err := routertest.FakeRouter.AddRoute(app.GetName(), cont.getAddress())
	c.Assert(err, check.IsNil)
	err = routertest.FakeRouter.AddRoute(app.GetName(), cont2.getAddress())
	c.Assert(err, check.IsNil)
	args := changeUnitsPipelineArgs{
		app:      app,
		toRemove: []container{cont, cont2},
	}
	context := action.FWContext{Previous: []container{}, Params: []interface{}{args}}
	r, err := removeOldRoutes.Forward(context)
	c.Assert(err, check.IsNil)
	hasRoute := routertest.FakeRouter.HasRoute(app.GetName(), cont.getAddress())
	c.Assert(hasRoute, check.Equals, false)
	hasRoute = routertest.FakeRouter.HasRoute(app.GetName(), cont2.getAddress())
	c.Assert(hasRoute, check.Equals, false)
	containers := r.([]container)
	c.Assert(containers, check.DeepEquals, []container{})
}

func (s *S) TestRemoveOldRoutesForwardFailInMiddle(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	routertest.FakeRouter.AddBackend(app.GetName())
	defer routertest.FakeRouter.RemoveBackend(app.GetName())
	cont := container{ID: "ble-1", AppName: app.GetName()}
	cont2 := container{ID: "ble-2", AppName: app.GetName()}
	defer cont.remove()
	defer cont2.remove()
	err := routertest.FakeRouter.AddRoute(app.GetName(), cont.getAddress())
	c.Assert(err, check.IsNil)
	err = routertest.FakeRouter.AddRoute(app.GetName(), cont2.getAddress())
	c.Assert(err, check.IsNil)
	routertest.FakeRouter.FailForIp(cont2.getAddress())
	args := changeUnitsPipelineArgs{
		app:      app,
		toRemove: []container{cont, cont2},
	}
	context := action.FWContext{Previous: []container{}, Params: []interface{}{args}}
	_, err = removeOldRoutes.Forward(context)
	c.Assert(err, check.Equals, routertest.ErrForcedFailure)
	hasRoute := routertest.FakeRouter.HasRoute(app.GetName(), cont.getAddress())
	c.Assert(hasRoute, check.Equals, true)
	hasRoute = routertest.FakeRouter.HasRoute(app.GetName(), cont2.getAddress())
	c.Assert(hasRoute, check.Equals, true)
}

func (s *S) TestRemoveOldRoutesBackward(c *check.C) {
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	routertest.FakeRouter.AddBackend(app.GetName())
	defer routertest.FakeRouter.RemoveBackend(app.GetName())
	cont := container{ID: "ble-1", AppName: app.GetName()}
	cont2 := container{ID: "ble-2", AppName: app.GetName()}
	defer cont.remove()
	defer cont2.remove()
	args := changeUnitsPipelineArgs{
		app:      app,
		toRemove: []container{cont, cont2},
	}
	context := action.BWContext{Params: []interface{}{args}}
	removeOldRoutes.Backward(context)
	hasRoute := routertest.FakeRouter.HasRoute(app.GetName(), cont.getAddress())
	c.Assert(hasRoute, check.Equals, true)
	hasRoute = routertest.FakeRouter.HasRoute(app.GetName(), cont2.getAddress())
	c.Assert(hasRoute, check.Equals, true)
}

func (s *S) TestSetNetworkInfoName(c *check.C) {
	c.Assert(setNetworkInfo.Name, check.Equals, "set-network-info")
}

func (s *S) TestSetNetworkInfoForward(c *check.C) {
	conta, err := s.newContainer(nil)
	c.Assert(err, check.IsNil)
	defer s.removeTestContainer(conta)
	cont := *conta
	context := action.FWContext{Previous: cont}
	r, err := setNetworkInfo.Forward(context)
	c.Assert(err, check.IsNil)
	cont = r.(container)
	c.Assert(cont, check.FitsTypeOf, container{})
	c.Assert(cont.IP, check.Not(check.Equals), "")
	c.Assert(cont.HostPort, check.Not(check.Equals), "")
}

func (s *S) TestSetImage(c *check.C) {
	conta, err := s.newContainer(nil)
	c.Assert(err, check.IsNil)
	defer s.removeTestContainer(conta)
	cont := *conta
	context := action.FWContext{Previous: cont}
	r, err := setNetworkInfo.Forward(context)
	c.Assert(err, check.IsNil)
	cont = r.(container)
	c.Assert(cont, check.FitsTypeOf, container{})
	c.Assert(cont.HostPort, check.Not(check.Equals), "")
}

func (s *S) TestStartContainerForward(c *check.C) {
	conta, err := s.newContainer(nil)
	c.Assert(err, check.IsNil)
	defer s.removeTestContainer(conta)
	cont := *conta
	context := action.FWContext{Previous: cont, Params: []interface{}{runContainerActionsArgs{}}}
	r, err := startContainer.Forward(context)
	c.Assert(err, check.IsNil)
	cont = r.(container)
	c.Assert(cont, check.FitsTypeOf, container{})
}

func (s *S) TestStartContainerBackward(c *check.C) {
	dcli, err := docker.NewClient(s.server.URL())
	c.Assert(err, check.IsNil)
	err = newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	defer dcli.RemoveImage("tsuru/python")
	conta, err := s.newContainer(nil)
	c.Assert(err, check.IsNil)
	defer s.removeTestContainer(conta)
	cont := *conta
	err = dcli.StartContainer(cont.ID, nil)
	c.Assert(err, check.IsNil)
	context := action.BWContext{FWResult: cont}
	startContainer.Backward(context)
	cc, err := dcli.InspectContainer(cont.ID)
	c.Assert(err, check.IsNil)
	c.Assert(cc.State.Running, check.Equals, false)
}

func (s *S) TestProvisionAddUnitsToHostName(c *check.C) {
	c.Assert(provisionAddUnitsToHost.Name, check.Equals, "provision-add-units-to-host")
}

func (s *S) TestProvisionAddUnitsToHostForward(c *check.C) {
	cluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(cluster)
	var p dockerProvisioner
	app := provisiontest.NewFakeApp("myapp-2", "python", 0)
	defer p.Destroy(app)
	p.Provision(app)
	coll := collection()
	defer coll.Close()
	coll.Insert(container{ID: "container-id", AppName: app.GetName(), Version: "container-version", Image: "tsuru/python"})
	defer coll.RemoveAll(bson.M{"appname": app.GetName()})
	imageId, err := appNewImageName(app.GetName())
	c.Assert(err, check.IsNil)
	err = newImage(imageId, "")
	c.Assert(err, check.IsNil)
	args := changeUnitsPipelineArgs{
		app:        app,
		toHost:     "localhost",
		unitsToAdd: 2,
		imageId:    imageId,
	}
	context := action.FWContext{Params: []interface{}{args}}
	result, err := provisionAddUnitsToHost.Forward(context)
	c.Assert(err, check.IsNil)
	containers := result.([]container)
	c.Assert(containers, check.HasLen, 2)
	c.Assert(containers[0].HostAddr, check.Equals, "localhost")
	c.Assert(containers[1].HostAddr, check.Equals, "localhost")
	count, err := coll.Find(bson.M{"appname": app.GetName()}).Count()
	c.Assert(err, check.IsNil)
	c.Assert(count, check.Equals, 3)
}

func (s *S) TestProvisionAddUnitsToHostForwardWithoutHost(c *check.C) {
	oldCluster, err := s.startMultipleServersCluster()
	c.Assert(err, check.IsNil)
	defer s.stopMultipleServersCluster(oldCluster)
	var p dockerProvisioner
	app := provisiontest.NewFakeApp("myapp-2", "python", 0)
	defer p.Destroy(app)
	p.Provision(app)
	coll := collection()
	defer coll.Close()
	imageId, err := appNewImageName(app.GetName())
	c.Assert(err, check.IsNil)
	err = newImage(imageId, "")
	c.Assert(err, check.IsNil)
	args := changeUnitsPipelineArgs{
		app:        app,
		unitsToAdd: 3,
		imageId:    imageId,
	}
	context := action.FWContext{Params: []interface{}{args}}
	result, err := provisionAddUnitsToHost.Forward(context)
	c.Assert(err, check.IsNil)
	containers := result.([]container)
	c.Assert(containers, check.HasLen, 3)
	addrs := []string{containers[0].HostAddr, containers[1].HostAddr, containers[2].HostAddr}
	sort.Strings(addrs)
	isValid := reflect.DeepEqual(addrs, []string{"127.0.0.1", "localhost", "localhost"}) ||
		reflect.DeepEqual(addrs, []string{"127.0.0.1", "127.0.0.1", "localhost"})
	if !isValid {
		clusterNodes, _ := dockerCluster().UnfilteredNodes()
		c.Fatalf("Expected multiple hosts, got: %#v\nAvailable nodes: %#v", containers, clusterNodes)
	}
	count, err := coll.Find(bson.M{"appname": app.GetName()}).Count()
	c.Assert(err, check.IsNil)
	c.Assert(count, check.Equals, 3)
}

func (s *S) TestProvisionAddUnitsToHostBackward(c *check.C) {
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	var p dockerProvisioner
	app := provisiontest.NewFakeApp("myapp-xxx-1", "python", 0)
	defer p.Destroy(app)
	p.Provision(app)
	coll := collection()
	defer coll.Close()
	cont := container{ID: "container-id", AppName: app.GetName(), Version: "container-version", Image: "tsuru/python"}
	coll.Insert(cont)
	defer coll.RemoveAll(bson.M{"appname": app.GetName()})
	context := action.BWContext{FWResult: []container{cont}}
	provisionAddUnitsToHost.Backward(context)
	_, err = getContainer(cont.ID)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "not found")
}

func (s *S) TestProvisionRemoveOldUnitsName(c *check.C) {
	c.Assert(provisionRemoveOldUnits.Name, check.Equals, "provision-remove-old-units")
}

func (s *S) TestProvisionRemoveOldUnitsForward(c *check.C) {
	cont, err := s.newContainer(nil)
	c.Assert(err, check.IsNil)
	defer routertest.FakeRouter.RemoveBackend(cont.AppName)
	client, err := docker.NewClient(s.server.URL())
	c.Assert(err, check.IsNil)
	err = client.StartContainer(cont.ID, nil)
	c.Assert(err, check.IsNil)
	app := provisiontest.NewFakeApp(cont.AppName, "python", 0)
	unit := cont.asUnit(app)
	app.BindUnit(&unit)
	args := changeUnitsPipelineArgs{
		app:      app,
		toRemove: []container{*cont},
	}
	context := action.FWContext{Params: []interface{}{args}, Previous: []container{}}
	result, err := provisionRemoveOldUnits.Forward(context)
	c.Assert(err, check.IsNil)
	resultContainers := result.([]container)
	c.Assert(resultContainers, check.DeepEquals, []container{})
	_, err = getContainer(cont.ID)
	c.Assert(err, check.NotNil)
	c.Assert(app.HasBind(&unit), check.Equals, false)
}

func (s *S) TestFollowLogsAndCommitName(c *check.C) {
	c.Assert(followLogsAndCommit.Name, check.Equals, "follow-logs-and-commit")
}

func (s *S) TestFollowLogsAndCommitForward(c *check.C) {
	go s.stopContainers(1)
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	app := provisiontest.NewFakeApp("mightyapp", "python", 1)
	nextImgName, err := appNewImageName(app.GetName())
	c.Assert(err, check.IsNil)
	cont := container{AppName: "mightyapp", ID: "myid123", BuildingImage: nextImgName}
	err = cont.create(runContainerActionsArgs{app: app, imageID: "tsuru/python", commands: []string{"foo"}})
	c.Assert(err, check.IsNil)
	var buf bytes.Buffer
	args := runContainerActionsArgs{writer: &buf}
	context := action.FWContext{Params: []interface{}{args}, Previous: cont}
	imageId, err := followLogsAndCommit.Forward(context)
	c.Assert(err, check.IsNil)
	c.Assert(imageId, check.Equals, "tsuru/app-mightyapp:v1")
	c.Assert(buf.String(), check.Not(check.Equals), "")
	var dbCont container
	coll := collection()
	defer coll.Close()
	err = coll.Find(bson.M{"id": cont.ID}).One(&dbCont)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "not found")
	_, err = dockerCluster().InspectContainer(cont.ID)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Matches, "No such container.*")
	err = dockerCluster().RemoveImage("tsuru/app-mightyapp:v1")
	c.Assert(err, check.IsNil)
}

func (s *S) TestFollowLogsAndCommitForwardNonZeroStatus(c *check.C) {
	go s.stopContainers(1)
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	cont := container{AppName: "mightyapp"}
	err = cont.create(runContainerActionsArgs{app: app, imageID: "tsuru/python", commands: []string{"foo"}})
	c.Assert(err, check.IsNil)
	err = s.server.MutateContainer(cont.ID, docker.State{ExitCode: 1})
	c.Assert(err, check.IsNil)
	var buf bytes.Buffer
	args := runContainerActionsArgs{writer: &buf}
	context := action.FWContext{Params: []interface{}{args}, Previous: cont}
	imageId, err := followLogsAndCommit.Forward(context)
	c.Assert(err, check.NotNil)
	c.Assert(err.Error(), check.Equals, "Exit status 1")
	c.Assert(imageId, check.IsNil)
}

func (s *S) TestFollowLogsAndCommitForwardWaitFailure(c *check.C) {
	s.server.PrepareFailure("failed to wait for the container", "/containers/.*/wait")
	defer s.server.ResetFailure("failed to wait for the container")
	err := newImage("tsuru/python", s.server.URL())
	c.Assert(err, check.IsNil)
	app := provisiontest.NewFakeApp("myapp", "python", 1)
	cont := container{AppName: "mightyapp"}
	err = cont.create(runContainerActionsArgs{app: app, imageID: "tsuru/python", commands: []string{"foo"}})
	c.Assert(err, check.IsNil)
	var buf bytes.Buffer
	args := runContainerActionsArgs{writer: &buf}
	context := action.FWContext{Params: []interface{}{args}, Previous: cont}
	imageId, err := followLogsAndCommit.Forward(context)
	c.Assert(err, check.ErrorMatches, `.*failed to wait for the container\n$`)
	c.Assert(imageId, check.IsNil)
}
