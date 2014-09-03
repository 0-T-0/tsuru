// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	dtesting "github.com/fsouza/go-dockerclient/testing"
	"github.com/tsuru/config"
	"github.com/tsuru/docker-cluster/cluster"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/iaas"
	"github.com/tsuru/tsuru/testing"
	"gopkg.in/mgo.v2/bson"
	"launchpad.net/gocheck"
)

type TestHealerIaaS struct {
	addr   string
	err    error
	delErr error
}

func (t *TestHealerIaaS) DeleteMachine(m *iaas.Machine) error {
	if t.delErr != nil {
		return t.delErr
	}
	return nil
}

func (t *TestHealerIaaS) CreateMachine(params map[string]string) (*iaas.Machine, error) {
	if t.err != nil {
		return nil, t.err
	}
	m := iaas.Machine{
		Id:      "m-" + t.addr,
		Status:  "running",
		Address: t.addr,
	}
	return &m, nil
}

func (TestHealerIaaS) Describe() string {
	return "iaas describe"
}

func urlPort(uStr string) int {
	url, _ := url.Parse(uStr)
	_, port, _ := net.SplitHostPort(url.Host)
	portInt, _ := strconv.Atoi(port)
	return portInt
}

func (s *S) TestHealerHealNode(c *gocheck.C) {
	oldCluster := dCluster
	defer func() {
		cmutex.Lock()
		defer cmutex.Unlock()
		machines, _ := iaas.ListMachines()
		for _, m := range machines {
			m.Destroy()
		}
		dCluster = oldCluster
	}()
	iaasInstance := &TestHealerIaaS{}
	iaas.RegisterIaasProvider("my-healer-iaas", iaasInstance)
	iaasInstance.addr = "127.0.0.1"
	_, err := iaas.CreateMachineForIaaS("my-healer-iaas", map[string]string{})
	c.Assert(err, gocheck.IsNil)
	iaasInstance.addr = "localhost"
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	node2, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	config.Set("iaas:node-protocol", "http")
	config.Set("iaas:node-port", urlPort(node2.URL()))
	defer config.Unset("iaas:node-protocol")
	defer config.Unset("iaas:node-port")
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	dCluster = cluster

	appInstance := testing.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	_, err = addContainersWithHost(nil, appInstance, 1, "127.0.0.1")
	c.Assert(err, gocheck.IsNil)

	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})

	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")

	containers, err := listAllContainers()
	c.Assert(err, gocheck.IsNil)
	c.Assert(containers, gocheck.HasLen, 1)
	c.Assert(containers[0].HostAddr, gocheck.Equals, "127.0.0.1")

	machines, err := iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 1)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")

	nodes[0].Metadata["iaas"] = "my-healer-iaas"
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.IsNil)
	c.Assert(created, gocheck.Equals, true)
	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node2.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "localhost")

	machines, err = iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 1)
	c.Assert(machines[0].Address, gocheck.Equals, "localhost")

	containers, err = listAllContainers()
	c.Assert(err, gocheck.IsNil)
	c.Assert(containers, gocheck.HasLen, 1)
	c.Assert(containers[0].HostAddr, gocheck.Equals, "localhost")
}

func (s *S) TestHealerHealNodeWithoutIaaS(c *gocheck.C) {
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.ErrorMatches, ".*no IaaS information.*")
	c.Assert(created, gocheck.Equals, false)
	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
}

func (s *S) TestHealerHealNodeCreateMachineError(c *gocheck.C) {
	defer func() {
		machines, _ := iaas.ListMachines()
		for _, m := range machines {
			m.Destroy()
		}
	}()
	iaasInstance := &TestHealerIaaS{}
	iaas.RegisterIaasProvider("my-healer-iaas", iaasInstance)
	iaasInstance.addr = "127.0.0.1"
	_, err := iaas.CreateMachineForIaaS("my-healer-iaas", map[string]string{})
	c.Assert(err, gocheck.IsNil)
	iaasInstance.err = fmt.Errorf("my create machine error")
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
	nodes[0].Metadata["iaas"] = "my-healer-iaas"
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.ErrorMatches, ".*my create machine error.*")
	c.Assert(created, gocheck.Equals, false)
	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
}

func (s *S) TestHealerHealNodeFormatError(c *gocheck.C) {
	defer func() {
		machines, _ := iaas.ListMachines()
		for _, m := range machines {
			m.Destroy()
		}
	}()
	iaasInstance := &TestHealerIaaS{}
	iaas.RegisterIaasProvider("my-healer-iaas", iaasInstance)
	iaasInstance.addr = "127.0.0.1"
	_, err := iaas.CreateMachineForIaaS("my-healer-iaas", map[string]string{})
	c.Assert(err, gocheck.IsNil)
	iaasInstance.addr = "localhost"
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
	nodes[0].Metadata["iaas"] = "my-healer-iaas"
	machines, err := iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 1)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.ErrorMatches, ".*error formatting address.*")
	c.Assert(created, gocheck.Equals, false)
	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
	machines, err = iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 1)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")
}

func (s *S) TestHealerHealNodeWaitAndRegisterError(c *gocheck.C) {
	defer func() {
		machines, _ := iaas.ListMachines()
		for _, m := range machines {
			m.Destroy()
		}
	}()
	iaasInstance := &TestHealerIaaS{}
	iaas.RegisterIaasProvider("my-healer-iaas", iaasInstance)
	iaasInstance.addr = "127.0.0.1"
	_, err := iaas.CreateMachineForIaaS("my-healer-iaas", map[string]string{})
	c.Assert(err, gocheck.IsNil)
	iaasInstance.addr = "localhost"
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	node2, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	node2.PrepareFailure("ping-failure", "/_ping")
	config.Set("iaas:node-protocol", "http")
	config.Set("iaas:node-port", urlPort(node2.URL()))
	defer config.Unset("iaas:node-protocol")
	defer config.Unset("iaas:node-port")
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
	nodes[0].Metadata["iaas"] = "my-healer-iaas"
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.ErrorMatches, ".*error registering new node.*")
	c.Assert(created, gocheck.Equals, false)
	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")
}

func (s *S) TestHealerHealNodeMoveFailure(c *gocheck.C) {
	oldCluster := dCluster
	defer func() {
		cmutex.Lock()
		defer cmutex.Unlock()
		machines, _ := iaas.ListMachines()
		for _, m := range machines {
			m.Destroy()
		}
		dCluster = oldCluster
	}()
	iaasInstance := &TestHealerIaaS{}
	iaas.RegisterIaasProvider("my-healer-iaas", iaasInstance)
	iaasInstance.addr = "127.0.0.1"
	_, err := iaas.CreateMachineForIaaS("my-healer-iaas", map[string]string{})
	c.Assert(err, gocheck.IsNil)
	iaasInstance.addr = "localhost"
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	node2, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	node2.PrepareFailure("create-error", "/containers/create")
	config.Set("iaas:node-protocol", "http")
	config.Set("iaas:node-port", urlPort(node2.URL()))
	defer config.Unset("iaas:node-protocol")
	defer config.Unset("iaas:node-port")
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	dCluster = cluster

	appInstance := testing.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	_, err = addContainersWithHost(nil, appInstance, 1, "127.0.0.1")
	c.Assert(err, gocheck.IsNil)

	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})

	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")

	containers, err := listAllContainers()
	c.Assert(err, gocheck.IsNil)
	c.Assert(containers, gocheck.HasLen, 1)
	c.Assert(containers[0].HostAddr, gocheck.Equals, "127.0.0.1")

	machines, err := iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 1)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")

	nodes[0].Metadata["iaas"] = "my-healer-iaas"
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.ErrorMatches, "(?s)Unable to move containers.*")
	c.Assert(created, gocheck.Equals, true)

	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node2.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "localhost")

	machines, err = iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 2)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")
	c.Assert(machines[1].Address, gocheck.Equals, "localhost")

	containers, err = listAllContainers()
	c.Assert(err, gocheck.IsNil)
	c.Assert(containers, gocheck.HasLen, 1)
	c.Assert(containers[0].HostAddr, gocheck.Equals, "127.0.0.1")
}

func (s *S) TestHealerHealNodeDestroyError(c *gocheck.C) {
	iaasInstance := &TestHealerIaaS{}
	oldCluster := dCluster
	defer func() {
		cmutex.Lock()
		defer cmutex.Unlock()
		iaasInstance.delErr = nil
		machines, _ := iaas.ListMachines()
		for _, m := range machines {
			m.Destroy()
		}
		dCluster = oldCluster
		machines, _ = iaas.ListMachines()
	}()
	iaasInstance.delErr = fmt.Errorf("my destroy error")
	iaas.RegisterIaasProvider("my-healer-iaas", iaasInstance)
	iaasInstance.addr = "127.0.0.1"
	_, err := iaas.CreateMachineForIaaS("my-healer-iaas", map[string]string{})
	c.Assert(err, gocheck.IsNil)
	iaasInstance.addr = "localhost"
	node1, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	node2, err := dtesting.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, gocheck.IsNil)
	config.Set("iaas:node-protocol", "http")
	config.Set("iaas:node-port", urlPort(node2.URL()))
	defer config.Unset("iaas:node-protocol")
	defer config.Unset("iaas:node-port")
	cluster, err := cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: node1.URL()},
	)
	c.Assert(err, gocheck.IsNil)
	dCluster = cluster

	appInstance := testing.NewFakeApp("myapp", "python", 0)
	var p dockerProvisioner
	defer p.Destroy(appInstance)
	p.Provision(appInstance)
	_, err = addContainersWithHost(nil, appInstance, 1, "127.0.0.1")
	c.Assert(err, gocheck.IsNil)

	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	appStruct := &app.App{
		Name: appInstance.GetName(),
	}
	err = conn.Apps().Insert(appStruct)
	c.Assert(err, gocheck.IsNil)
	defer conn.Apps().Remove(bson.M{"name": appStruct.Name})

	healer := Healer{
		cluster:               cluster,
		disabledTime:          0,
		failuresBeforeHealing: 1,
		waitTimeNewMachine:    1 * time.Second,
	}
	nodes, err := cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node1.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "127.0.0.1")

	containers, err := listAllContainers()
	c.Assert(err, gocheck.IsNil)
	c.Assert(containers, gocheck.HasLen, 1)
	c.Assert(containers[0].HostAddr, gocheck.Equals, "127.0.0.1")

	machines, err := iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 1)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")

	nodes[0].Metadata["iaas"] = "my-healer-iaas"
	created, err := healer.healNode(nodes[0])
	c.Assert(err, gocheck.ErrorMatches, "(?s)Unable to destroy machine.*my destroy error")
	c.Assert(created, gocheck.Equals, true)

	nodes, err = cluster.UnfilteredNodes()
	c.Assert(err, gocheck.IsNil)
	c.Assert(nodes, gocheck.HasLen, 1)
	c.Assert(urlPort(nodes[0].Address), gocheck.Equals, urlPort(node2.URL()))
	c.Assert(urlToHost(nodes[0].Address), gocheck.Equals, "localhost")

	machines, err = iaas.ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 2)
	c.Assert(machines[0].Address, gocheck.Equals, "127.0.0.1")
	c.Assert(machines[1].Address, gocheck.Equals, "localhost")

	containers, err = listAllContainers()
	c.Assert(err, gocheck.IsNil)
	c.Assert(containers, gocheck.HasLen, 1)
	c.Assert(containers[0].HostAddr, gocheck.Equals, "localhost")
}
