// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iaas

import (
	"launchpad.net/gocheck"
	"testing"
)

func Test(t *testing.T) { gocheck.TestingT(t) }

type S struct{}

var _ = gocheck.Suite(&S{})

func (s *S) SetUpSuite(c *gocheck.C) {
	RegisterIaasProvider("test-iaas", TestIaaS{})
}

func (s *S) SetUpTest(c *gocheck.C) {
	coll := collection()
	defer coll.Close()
	coll.RemoveAll(nil)
}

type TestIaaS struct{}

func (TestIaaS) DeleteMachine(m *Machine) error {
	m.Status = "destroyed"
	return nil
}

func (TestIaaS) CreateMachine(params map[string]string) (*Machine, error) {
	m := Machine{
		Id:      params["id"],
		Status:  "running",
		Address: params["id"] + ".somewhere.com",
	}
	return &m, nil
}
