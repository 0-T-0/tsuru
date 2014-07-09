// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iaas

import (
	"github.com/tsuru/config"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
)

func (s *S) TestCreateMachineForIaaS(c *gocheck.C) {
	m, err := CreateMachineForIaaS("test-iaas", map[string]string{"id": "myid"})
	c.Assert(err, gocheck.IsNil)
	c.Assert(m.Id, gocheck.Equals, "myid")
	c.Assert(m.Iaas, gocheck.Equals, "test-iaas")
	coll := collection()
	defer coll.Close()
	var dbMachine Machine
	err = coll.Find(bson.M{"_id": "myid"}).One(&dbMachine)
	c.Assert(err, gocheck.IsNil)
	c.Assert(dbMachine.Id, gocheck.Equals, "myid")
	c.Assert(dbMachine.Iaas, gocheck.Equals, "test-iaas")
}

func (s *S) TestCreateMachine(c *gocheck.C) {
	config.Set("iaas:default", "test-iaas")
	m, err := CreateMachine(map[string]string{"id": "myid"})
	c.Assert(err, gocheck.IsNil)
	c.Assert(m.Id, gocheck.Equals, "myid")
	c.Assert(m.Iaas, gocheck.Equals, "test-iaas")
}

func (s *S) TestListMachines(c *gocheck.C) {
	_, err := CreateMachineForIaaS("test-iaas", map[string]string{"id": "myid1"})
	c.Assert(err, gocheck.IsNil)
	_, err = CreateMachineForIaaS("test-iaas", map[string]string{"id": "myid2"})
	c.Assert(err, gocheck.IsNil)
	machines, err := ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 2)
	c.Assert(machines[0].Id, gocheck.Equals, "myid1")
	c.Assert(machines[1].Id, gocheck.Equals, "myid2")
}

func (s *S) TestDestroy(c *gocheck.C) {
	m, err := CreateMachineForIaaS("test-iaas", map[string]string{"id": "myid1"})
	c.Assert(err, gocheck.IsNil)
	err = m.Destroy()
	c.Assert(err, gocheck.IsNil)
	c.Assert(m.Status, gocheck.Equals, "destroyed")
	machines, err := ListMachines()
	c.Assert(err, gocheck.IsNil)
	c.Assert(machines, gocheck.HasLen, 0)
}
