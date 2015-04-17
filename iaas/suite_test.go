// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iaas

import (
	"testing"

	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func (s *S) SetUpTest(c *check.C) {
	iaasProviders = make(map[string]iaasFactory)
	iaasInstances = make(map[string]IaaS)
	RegisterIaasProvider("test-iaas", newTestIaaS)
	coll, err := collection()
	c.Assert(err, check.IsNil)
	defer coll.Close()
	coll.RemoveAll(nil)
	tplColl := template_collection()
	defer tplColl.Close()
	tplColl.RemoveAll(nil)
}

type TestIaaS struct {
	someField string
}

func (*TestIaaS) DeleteMachine(m *Machine) error {
	m.Status = "destroyed"
	return nil
}

func (*TestIaaS) CreateMachine(params map[string]string) (*Machine, error) {
	params["should"] = "be in"
	m := Machine{
		Id:      params["id"],
		Status:  "running",
		Address: params["id"] + ".somewhere.com",
	}
	return &m, nil
}

type TestDescriberIaaS struct {
	TestIaaS
}

func (i TestDescriberIaaS) DeleteMachine(m *Machine) error {
	return i.TestIaaS.DeleteMachine(m)
}

func (i TestDescriberIaaS) CreateMachine(params map[string]string) (*Machine, error) {
	return i.TestIaaS.CreateMachine(params)
}

func (i TestDescriberIaaS) Describe() string {
	return "ahoy desc!"
}

type TestCustomizableIaaS struct {
	NamedIaaS
	TestIaaS
}

func (i TestCustomizableIaaS) DeleteMachine(m *Machine) error {
	return i.TestIaaS.DeleteMachine(m)
}

func (i TestCustomizableIaaS) CreateMachine(params map[string]string) (*Machine, error) {
	return i.TestIaaS.CreateMachine(params)
}

type TestHealthCheckerIaaS struct {
	TestIaaS
	err error
}

func (i *TestHealthCheckerIaaS) HealthCheck() error {
	return i.err
}

func newTestHealthcheckIaaS(name string) IaaS {
	return &TestHealthCheckerIaaS{}
}

func newTestCustomizableIaaS(name string) IaaS {
	return TestCustomizableIaaS{NamedIaaS: NamedIaaS{IaaSName: name}}
}

func newTestDescriberIaaS(name string) IaaS {
	return TestDescriberIaaS{}
}

func newTestIaaS(name string) IaaS {
	return &TestIaaS{}
}
