// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/testing"
	"github.com/tsuru/config"
	"github.com/tsuru/docker-cluster/cluster"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/cmd"
	"github.com/tsuru/tsuru/db"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
	"sync"
)

type SchedulerSuite struct {
	storage *db.Storage
}

var _ = gocheck.Suite(&SchedulerSuite{})

func (s *SchedulerSuite) SetUpSuite(c *gocheck.C) {
	var err error
	config.Set("database:url", "127.0.0.1:27017")
	config.Set("database:name", "docker_scheduler_tests")
	config.Set("docker:repository-namespace", "tsuru")
	config.Set("docker:collection", "docker_unit_tests")
	s.storage, err = db.Conn()
	c.Assert(err, gocheck.IsNil)
}

func (s *SchedulerSuite) TearDownSuite(c *gocheck.C) {
	s.storage.Apps().Database.DropDatabase()
	s.storage.Close()
}

func (s *SchedulerSuite) TestSchedulerSchedule(c *gocheck.C) {
	server0, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server0.Stop()
	server1, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server1.Stop()
	server2, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server2.Stop()
	var buf bytes.Buffer
	client, _ := docker.NewClient(server0.URL())
	client.PullImage(
		docker.PullImageOptions{Repository: "tsuru/python", OutputStream: &buf},
		docker.AuthConfiguration{},
	)
	client, _ = docker.NewClient(server1.URL())
	client.PullImage(
		docker.PullImageOptions{Repository: "tsuru/python", OutputStream: &buf},
		docker.AuthConfiguration{},
	)
	client, _ = docker.NewClient(server2.URL())
	client.PullImage(
		docker.PullImageOptions{Repository: "tsuru/python", OutputStream: &buf},
		docker.AuthConfiguration{},
	)
	a1 := app.App{Name: "impius", Teams: []string{"tsuruteam", "nodockerforme"}}
	a2 := app.App{Name: "mirror", Teams: []string{"tsuruteam"}}
	a3 := app.App{Name: "dedication", Teams: []string{"nodockerforme"}}
	cont1 := container{ID: "1", Name: "impius1", AppName: a1.Name}
	cont2 := container{ID: "2", Name: "mirror1", AppName: a2.Name}
	cont3 := container{ID: "3", Name: "dedication1", AppName: a3.Name}
	err = s.storage.Apps().Insert(a1, a2, a3)
	c.Assert(err, gocheck.IsNil)
	defer s.storage.Apps().RemoveAll(bson.M{"name": bson.M{"$in": []string{a1.Name, a2.Name, a3.Name}}})
	coll := s.storage.Collection(schedulerCollection)
	err = coll.Insert(
		node{ID: "server0", Address: server0.URL(), Teams: []string{"tsuruteam"}},
		node{ID: "server1", Address: server1.URL(), Teams: []string{"tsuruteam"}},
		node{ID: "server2", Address: server2.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": bson.M{"$in": []string{"server0", "server1", "server2"}}})
	contColl := collection()
	err = contColl.Insert(
		cont1, cont2, cont3,
	)
	c.Assert(err, gocheck.IsNil)
	defer contColl.RemoveAll(bson.M{"name": bson.M{"$in": []string{cont1.Name, cont2.Name, cont3.Name}}})
	var scheduler segregatedScheduler
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	opts := docker.CreateContainerOptions{Name: cont1.Name, Config: &config}
	node, _, err := scheduler.Schedule(opts)
	c.Assert(err, gocheck.IsNil)
	c.Check(node, gocheck.Equals, "server1")
	config = docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	opts = docker.CreateContainerOptions{Name: cont2.Name, Config: &config}
	node, _, err = scheduler.Schedule(opts)
	c.Assert(err, gocheck.IsNil)
	c.Check(node == "server0" || node == "server1", gocheck.Equals, true)
	config = docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	opts = docker.CreateContainerOptions{Name: cont3.Name, Config: &config}
	node, _, err = scheduler.Schedule(opts)
	c.Assert(err, gocheck.IsNil)
	c.Check(node, gocheck.Equals, "server2")
}

func (s *SchedulerSuite) TestSchedulerScheduleWithRegistry(c *gocheck.C) {
	config.Set("docker:registry", "myregistry.secret.server.com")
	defer config.Unset("docker:registry")
	server0, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server0.Stop()
	server1, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server1.Stop()
	server2, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server2.Stop()
	a1 := app.App{Name: "impius", Teams: []string{"tsuruteam", "nodockerforme"}}
	a2 := app.App{Name: "mirror", Teams: []string{"tsuruteam"}}
	a3 := app.App{Name: "dedication", Teams: []string{"nodockerforme"}}
	cont1 := container{ID: "1", Name: "impius1", AppName: a1.Name}
	cont2 := container{ID: "2", Name: "mirror1", AppName: a2.Name}
	cont3 := container{ID: "3", Name: "dedication1", AppName: a3.Name}
	err = s.storage.Apps().Insert(a1, a2, a3)
	c.Assert(err, gocheck.IsNil)
	defer s.storage.Apps().RemoveAll(bson.M{"name": bson.M{"$in": []string{a1.Name, a2.Name, a3.Name}}})
	coll := s.storage.Collection(schedulerCollection)
	err = coll.Insert(
		node{ID: "server0", Address: server0.URL(), Teams: []string{"tsuruteam"}},
		node{ID: "server1", Address: server1.URL(), Teams: []string{"tsuruteam"}},
		node{ID: "server2", Address: server2.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": bson.M{"$in": []string{"server0", "server1", "server2"}}})
	contColl := collection()
	err = contColl.Insert(
		cont1, cont2, cont3,
	)
	c.Assert(err, gocheck.IsNil)
	defer contColl.RemoveAll(bson.M{"name": bson.M{"$in": []string{cont1.Name, cont2.Name, cont3.Name}}})
	var scheduler segregatedScheduler
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	opts := docker.CreateContainerOptions{Name: cont1.Name, Config: &config}
	node, _, err := scheduler.Schedule(opts)
	c.Assert(err, gocheck.IsNil)
	c.Check(node == "server1" || node == "server0", gocheck.Equals, true)
}

func (s *SchedulerSuite) TestSchedulerScheduleFallback(c *gocheck.C) {
	server0, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server0.Stop()
	var buf bytes.Buffer
	client, _ := docker.NewClient(server0.URL())
	client.PullImage(
		docker.PullImageOptions{Repository: "tsuru/python", OutputStream: &buf},
		docker.AuthConfiguration{},
	)
	a1 := app.App{Name: "impius", Teams: []string{"tsuruteam", "nodockerforme"}}
	cont1 := container{ID: "1", Name: "impius1", AppName: a1.Name}
	err = s.storage.Apps().Insert(a1)
	c.Assert(err, gocheck.IsNil)
	defer s.storage.Apps().RemoveAll(bson.M{"name": a1.Name})
	coll := s.storage.Collection(schedulerCollection)
	err = coll.Insert(
		bson.M{"_id": "server0", "address": server0.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": "server0"})
	contColl := collection()
	err = contColl.Insert(cont1)
	c.Assert(err, gocheck.IsNil)
	defer contColl.RemoveAll(bson.M{"name": cont1.Name})
	var scheduler segregatedScheduler
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	opts := docker.CreateContainerOptions{Name: cont1.Name, Config: &config}
	node, _, err := scheduler.Schedule(opts)
	c.Assert(err, gocheck.IsNil)
	c.Check(node, gocheck.Equals, "server0")
}

func (s *SchedulerSuite) TestSchedulerScheduleTeamOwner(c *gocheck.C) {
	server0, err := testing.NewServer("127.0.0.1:0", nil)
	c.Assert(err, gocheck.IsNil)
	defer server0.Stop()
	var buf bytes.Buffer
	client, _ := docker.NewClient(server0.URL())
	client.PullImage(
		docker.PullImageOptions{Repository: "tsuru/python", OutputStream: &buf},
		docker.AuthConfiguration{},
	)
	a1 := app.App{
		Name:      "impius",
		Teams:     []string{"nodockerforme"},
		TeamOwner: "tsuruteam",
	}
	cont1 := container{ID: "1", Name: "impius1", AppName: a1.Name}
	err = s.storage.Apps().Insert(a1)
	c.Assert(err, gocheck.IsNil)
	defer s.storage.Apps().RemoveAll(bson.M{"name": a1.Name})
	coll := s.storage.Collection(schedulerCollection)
	err = coll.Insert(
		bson.M{"_id": "server0", "address": server0.URL(), "teams": []string{"tsuruteam"}},
	)
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": "server0"})
	contColl := collection()
	err = contColl.Insert(cont1)
	c.Assert(err, gocheck.IsNil)
	defer contColl.RemoveAll(bson.M{"name": cont1.Name})
	var scheduler segregatedScheduler
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	opts := docker.CreateContainerOptions{Name: cont1.Name, Config: &config}
	node, _, err := scheduler.Schedule(opts)
	c.Assert(err, gocheck.IsNil)
	c.Check(node, gocheck.Equals, "server0")
}

func (s *SchedulerSuite) TestSchedulerNoFallback(c *gocheck.C) {
	app := app.App{Name: "bill", Teams: []string{"jean"}}
	err := s.storage.Apps().Insert(app)
	c.Assert(err, gocheck.IsNil)
	defer s.storage.Apps().Remove(bson.M{"name": app.Name})
	cont1 := container{ID: "1", Name: "bill", AppName: app.Name}
	contColl := collection()
	err = contColl.Insert(cont1)
	c.Assert(err, gocheck.IsNil)
	defer contColl.Remove(bson.M{"name": cont1.Name})
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	var scheduler segregatedScheduler
	opts := docker.CreateContainerOptions{Name: cont1.Name, Config: &config}
	node, container, err := scheduler.Schedule(opts)
	c.Assert(node, gocheck.Equals, "")
	c.Assert(container, gocheck.IsNil)
	c.Assert(err, gocheck.Equals, errNoFallback)
}

func (s *SchedulerSuite) TestSchedulerNoNamespace(c *gocheck.C) {
	old, _ := config.Get("docker:repository-namespace")
	defer config.Set("docker:repository-namespace", old)
	config.Unset("docker:repository-namespace")
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/python"}
	var scheduler segregatedScheduler
	opts := docker.CreateContainerOptions{Config: &config}
	node, container, err := scheduler.Schedule(opts)
	c.Assert(node, gocheck.Equals, "")
	c.Assert(container, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
}

func (s *SchedulerSuite) TestSchedulerInvalidEndpoint(c *gocheck.C) {
	app := app.App{Name: "bill", Teams: []string{"jean"}}
	err := s.storage.Apps().Insert(app)
	c.Assert(err, gocheck.IsNil)
	defer s.storage.Apps().Remove(bson.M{"name": app.Name})
	cont1 := container{ID: "1", Name: "bill", AppName: app.Name}
	contColl := collection()
	err = contColl.Insert(cont1)
	c.Assert(err, gocheck.IsNil)
	defer contColl.Remove(bson.M{"name": cont1.Name})
	coll := s.storage.Collection(schedulerCollection)
	err = coll.Insert(node{ID: "server0", Address: "", Teams: []string{"jean"}})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"_id": "server0"})
	config := docker.Config{Cmd: []string{"/usr/sbin/sshd", "-D"}, Image: "tsuru/bill"}
	var scheduler segregatedScheduler
	opts := docker.CreateContainerOptions{Name: cont1.Name, Config: &config}
	node, container, err := scheduler.Schedule(opts)
	c.Assert(node, gocheck.Equals, "server0")
	c.Assert(container, gocheck.IsNil)
	c.Assert(err, gocheck.NotNil)
}

func (s *SchedulerSuite) TestSchedulerNodes(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	err := coll.Insert(
		node{ID: "server0", Address: "http://localhost:8080", Teams: []string{"tsuru"}},
		node{ID: "server1", Address: "http://localhost:8081", Teams: []string{"tsuru"}},
		node{ID: "server2", Address: "http://localhost:8082", Teams: []string{"tsuru"}},
	)
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": bson.M{"$in": []string{"server0", "server1", "server2"}}})
	expected := []cluster.Node{
		{ID: "server0", Address: "http://localhost:8080"},
		{ID: "server1", Address: "http://localhost:8081"},
		{ID: "server2", Address: "http://localhost:8082"},
	}
	var scheduler segregatedScheduler
	nodes, err := scheduler.Nodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.DeepEquals, expected)
}

func (s *SchedulerSuite) TestSchedulerGetNode(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	err := coll.Insert(
		node{ID: "server0", Address: "http://localhost:8080", Teams: []string{"tsuru"}},
		node{ID: "server1", Address: "http://localhost:8081", Teams: []string{"tsuru"}},
		node{ID: "server2", Address: "http://localhost:8082", Teams: []string{"tsuru"}},
	)
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": bson.M{"$in": []string{"server0", "server1", "server2"}}})
	var tests = []struct {
		input    string
		expected node
		err      error
	}{
		{"server0", node{ID: "server0", Address: "http://localhost:8080", Teams: []string{"tsuru"}}, nil},
		{"server1", node{ID: "server1", Address: "http://localhost:8081", Teams: []string{"tsuru"}}, nil},
		{"server2", node{ID: "server2", Address: "http://localhost:8082", Teams: []string{"tsuru"}}, nil},
		{"server102", node{}, errNodeNotFound},
	}
	var scheduler segregatedScheduler
	for _, t := range tests {
		nd, err := scheduler.GetNode(t.input)
		c.Check(err, gocheck.Equals, t.err)
		c.Check(nd, gocheck.DeepEquals, t.expected)
	}
}

func (s *SchedulerSuite) TestAddNodeToScheduler(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	nd := cluster.Node{ID: "server0", Address: "http://localhost:8080"}
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": nd.ID, "address": nd.Address, "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": nd.ID})
	var n node
	err = coll.Find(bson.M{"_id": nd.ID}).One(&n)
	c.Assert(err, gocheck.IsNil)
	c.Check(n.ID, gocheck.Equals, nd.ID)
	c.Check(n.Teams, gocheck.DeepEquals, []string{"team1"})
	c.Check(n.Address, gocheck.Equals, "http://localhost:8080")
}

func (s *SchedulerSuite) TestAddNodeDuplicated(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	nd := cluster.Node{ID: "server0", Address: "http://localhost:8080"}
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": nd.ID, "address": nd.Address, "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": nd.ID})
	err = scheduler.Register(map[string]string{"ID": nd.ID, "address": nd.Address, "team": "team2"})
	c.Assert(err, gocheck.Equals, errNodeAlreadyRegister)
}

func (s *SchedulerSuite) TestRemoveNodeFromScheduler(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	nd := cluster.Node{ID: "server0", Address: "http://localhost:8080"}
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": nd.ID, "address": nd.Address, "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": "server0"})
	err = scheduler.Unregister(map[string]string{"ID": nd.ID})
	c.Assert(err, gocheck.IsNil)
	n, err := coll.Find(bson.M{"_id": "server0"}).Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 0)
}

func (s *SchedulerSuite) TesteRemoveUnknownNodeFromScheduler(c *gocheck.C) {
	var scheduler segregatedScheduler
	err := scheduler.Unregister(map[string]string{"ID": "server0"})
	c.Assert(err, gocheck.Equals, errNodeNotFound)
}

func (s *SchedulerSuite) TestListNodesInTheScheduler(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": "server0", "address": "http://localhost:8080", "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	err = scheduler.Register(map[string]string{"ID": "server1", "address": "http://localhost:9090", "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	err = scheduler.Register(map[string]string{"ID": "server2", "address": "http://localhost:9090", "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": bson.M{"$in": []string{"server0", "server1", "server2"}}})
	nodes, err := listNodesInTheScheduler()
	c.Assert(err, gocheck.IsNil)
	expected := []node{
		{ID: "server0", Address: "http://localhost:8080", Teams: []string{"team1"}},
		{ID: "server1", Address: "http://localhost:9090", Teams: []string{"team1"}},
		{ID: "server2", Address: "http://localhost:9090", Teams: []string{"team1"}},
	}
	c.Assert(nodes, gocheck.DeepEquals, expected)
}

func (s *SchedulerSuite) TestAddNodeToTheSchedulerCmdInfo(c *gocheck.C) {
	expected := cmd.Info{
		Name:    "docker-add-node",
		Usage:   "docker-add-node <id> <address> [team]",
		Desc:    "Registers a new node in the cluster, optionally assigning it to a team",
		MinArgs: 2,
	}
	cmd := addNodeToSchedulerCmd{}
	c.Assert(cmd.Info(), gocheck.DeepEquals, &expected)
}

func (s *SchedulerSuite) TestAddNodeToTheSchedulerCmdRun(c *gocheck.C) {
	var buf bytes.Buffer
	coll := s.storage.Collection(schedulerCollection)
	context := cmd.Context{Args: []string{"server0", "http://localhost:8080"}, Stdout: &buf}
	cmd := addNodeToSchedulerCmd{}
	err := cmd.Run(&context, nil)
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"_id": "server0"})
	var n node
	err = coll.Find(bson.M{"_id": "server0"}).One(&n)
	c.Assert(err, gocheck.IsNil)
	c.Check(n.ID, gocheck.Equals, "server0")
	c.Check(len(n.Teams), gocheck.Equals, 0)
	c.Check(n.Address, gocheck.Equals, "http://localhost:8080")
	c.Assert(buf.String(), gocheck.Equals, "Node successfully registered.\n")
}

func (s *SchedulerSuite) TestAddNodeToTheSchedulerCmdRunWithTeam(c *gocheck.C) {
	var buf bytes.Buffer
	coll := s.storage.Collection(schedulerCollection)
	context := cmd.Context{Args: []string{"server0", "http://localhost:8080", "team1"}, Stdout: &buf}
	cmd := addNodeToSchedulerCmd{}
	err := cmd.Run(&context, nil)
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"_id": "server0"})
	var n node
	err = coll.Find(bson.M{"_id": "server0"}).One(&n)
	c.Assert(err, gocheck.IsNil)
	c.Check(n.ID, gocheck.Equals, "server0")
	c.Check(n.Teams, gocheck.DeepEquals, []string{"team1"})
	c.Check(n.Address, gocheck.Equals, "http://localhost:8080")
	c.Assert(buf.String(), gocheck.Equals, "Node successfully registered.\n")
}

func (s *SchedulerSuite) TestAddNodeToTheSchedulerCmdFailure(c *gocheck.C) {
	var buf bytes.Buffer
	coll := s.storage.Collection(schedulerCollection)
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": "server0", "address": "http://localhost:4243", "team": ""})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"_id": "server0"})
	context := cmd.Context{Args: []string{"server0", "http://localhost:8080", "team1"}, Stdout: &buf}
	cmd := addNodeToSchedulerCmd{}
	err = cmd.Run(&context, nil)
	c.Assert(err, gocheck.NotNil)
}

func (s *SchedulerSuite) TestRemoveNodeFromTheSchedulerCmdInfo(c *gocheck.C) {
	expected := cmd.Info{
		Name:    "docker-rm-node",
		Usage:   "docker-rm-node <id>",
		Desc:    "Removes a node from the cluster",
		MinArgs: 1,
	}
	cmd := removeNodeFromSchedulerCmd{}
	c.Assert(cmd.Info(), gocheck.DeepEquals, &expected)
}

func (s *SchedulerSuite) TestRemoveNodeFromTheSchedulerCmdRun(c *gocheck.C) {
	var buf bytes.Buffer
	coll := s.storage.Collection(schedulerCollection)
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": "server0", "address": "http://localhost:8080", "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	defer coll.Remove(bson.M{"_id": "server0"})
	context := cmd.Context{Args: []string{"server0"}, Stdout: &buf}
	err = removeNodeFromSchedulerCmd{}.Run(&context, nil)
	c.Assert(err, gocheck.IsNil)
	n, err := coll.Find(bson.M{"_id": "server0"}).Count()
	c.Assert(err, gocheck.IsNil)
	c.Assert(n, gocheck.Equals, 0)
	c.Assert(buf.String(), gocheck.Equals, "Node successfully removed.\n")
}

func (s *SchedulerSuite) TestRemoveNodeFromTheSchedulerCmdRunFailure(c *gocheck.C) {
	var buf bytes.Buffer
	context := cmd.Context{Args: []string{"server0"}, Stdout: &buf}
	err := removeNodeFromSchedulerCmd{}.Run(&context, nil)
	c.Assert(err, gocheck.NotNil)
}

func (s *SchedulerSuite) TestListNodesInTheSchedulerCmdInfo(c *gocheck.C) {
	expected := cmd.Info{
		Name:  "docker-list-nodes",
		Usage: "docker-list-nodes",
		Desc:  "List available nodes in the cluster",
	}
	cmd := listNodesInTheSchedulerCmd{}
	c.Assert(cmd.Info(), gocheck.DeepEquals, &expected)
}

func (s *SchedulerSuite) TestListNodesInTheSchedulerCmdRun(c *gocheck.C) {
	coll := s.storage.Collection(schedulerCollection)
	var scheduler segregatedScheduler
	err := scheduler.Register(map[string]string{"ID": "server0", "address": "http://localhost:8080", "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	err = scheduler.Register(map[string]string{"ID": "server1", "address": "http://localhost:9090", "team": "team1"})
	c.Assert(err, gocheck.IsNil)
	err = scheduler.Register(map[string]string{"ID": "server2", "address": "http://localhost:9090", "team": ""})
	c.Assert(err, gocheck.IsNil)
	defer coll.RemoveAll(bson.M{"_id": bson.M{"$in": []string{"server0", "server1", "server2"}}})
	var buf bytes.Buffer
	ctx := cmd.Context{Stdout: &buf}
	err = listNodesInTheSchedulerCmd{}.Run(&ctx, nil)
	c.Assert(err, gocheck.IsNil)
	expected := `+---------+-----------------------+-------+
| ID      | Address               | Team  |
+---------+-----------------------+-------+
| server0 | http://localhost:8080 | team1 |
| server1 | http://localhost:9090 | team1 |
| server2 | http://localhost:9090 |       |
+---------+-----------------------+-------+
`
	c.Assert(buf.String(), gocheck.Equals, expected)
}

func (s *SchedulerSuite) TestChooseNodeDistributesNodesEqually(c *gocheck.C) {
	nodes := []node{
		{ID: "node1", Address: "http://server1:1234"},
		{ID: "node2", Address: "http://server2:1234"},
		{ID: "node3", Address: "http://server3:1234"},
		{ID: "node4", Address: "http://server4:1234"},
	}
	contColl := collection()
	defer contColl.RemoveAll(bson.M{"appname": "coolapp9"})
	cont1 := container{ID: "pre1", Name: "existingUnit1", AppName: "coolapp9", HostAddr: "server1"}
	err := contColl.Insert(cont1)
	c.Assert(err, gocheck.Equals, nil)
	cont2 := container{ID: "pre2", Name: "existingUnit2", AppName: "coolapp9", HostAddr: "server2"}
	err = contColl.Insert(cont2)
	c.Assert(err, gocheck.Equals, nil)
	numberOfUnits := 38
	unitsPerNode := (numberOfUnits + 2) / 4
	wg := sync.WaitGroup{}
	wg.Add(numberOfUnits)
	for i := 0; i < numberOfUnits; i++ {
		go func(i int) {
			defer wg.Done()
			cont := container{ID: string(i), Name: fmt.Sprintf("unit%d", i), AppName: "coolapp9"}
			err := contColl.Insert(cont)
			c.Assert(err, gocheck.IsNil)
			node, err := chooseNode(nodes, cont)
			c.Assert(err, gocheck.IsNil)
			c.Assert(node.ID, gocheck.NotNil)
		}(i)
	}
	wg.Wait()
	n, err := contColl.Find(bson.M{"hostaddr": "server1"}).Count()
	c.Assert(err, gocheck.Equals, nil)
	c.Check(n, gocheck.Equals, unitsPerNode)
	n, err = contColl.Find(bson.M{"hostaddr": "server2"}).Count()
	c.Assert(err, gocheck.Equals, nil)
	c.Check(n, gocheck.Equals, unitsPerNode)
	n, err = contColl.Find(bson.M{"hostaddr": "server3"}).Count()
	c.Assert(err, gocheck.Equals, nil)
	c.Check(n, gocheck.Equals, unitsPerNode)
	n, err = contColl.Find(bson.M{"hostaddr": "server4"}).Count()
	c.Assert(err, gocheck.Equals, nil)
	c.Check(n, gocheck.Equals, unitsPerNode)
}
