// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"github.com/tsuru/config"
	"github.com/tsuru/tsuru/db"
	"github.com/tsuru/tsuru/testing"
	"labix.org/v2/mgo/bson"
	"launchpad.net/gocheck"
)

type PlatformSuite struct {
	provisioner *testing.FakeProvisioner
}

var _ = gocheck.Suite(&PlatformSuite{})

func (s *PlatformSuite) SetUpSuite(c *gocheck.C) {
	config.Set("database:url", "127.0.0.1:27017")
	config.Set("database:name", "platform_tests")
	s.provisioner = testing.NewFakeProvisioner()
	Provisioner = s.provisioner
}

func (s *PlatformSuite) TearDownSuite(c *gocheck.C) {
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	conn.Apps().Database.DropDatabase()
	conn.Close()
}

func (s *PlatformSuite) TestPlatforms(c *gocheck.C) {
	want := []Platform{
		{Name: "dea"},
		{Name: "pecuniae"},
		{Name: "money"},
		{Name: "raise"},
		{Name: "glass"},
	}
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	for _, p := range want {
		conn.Platforms().Insert(p)
		defer conn.Platforms().Remove(p)
	}
	got, err := Platforms()
	c.Assert(err, gocheck.IsNil)
	c.Assert(got, gocheck.DeepEquals, want)
}

func (s *PlatformSuite) TestPlatformsEmpty(c *gocheck.C) {
	got, err := Platforms()
	c.Assert(err, gocheck.IsNil)
	c.Assert(got, gocheck.HasLen, 0)
}

func (s *PlatformSuite) TestGetPlatform(c *gocheck.C) {
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	p := Platform{Name: "dea"}
	conn.Platforms().Insert(p)
	defer conn.Platforms().Remove(p)
	got, err := getPlatform(p.Name)
	c.Assert(err, gocheck.IsNil)
	c.Assert(*got, gocheck.DeepEquals, p)
	got, err = getPlatform("WAT")
	c.Assert(got, gocheck.IsNil)
	_, ok := err.(InvalidPlatformError)
	c.Assert(ok, gocheck.Equals, true)
}

func (s *PlatformSuite) TestPlatformAdd(c *gocheck.C) {
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	name := "test_platform_add"
	args := make(map[string]string)
	args["dockerfile"] = "http://localhost/Dockerfile"
	err = PlatformAdd(name, args, nil)
	defer conn.Platforms().Remove(bson.M{"_id": name})
	c.Assert(err, gocheck.IsNil)
	err = PlatformAdd(name, args, nil)
	_, ok := err.(DuplicatePlatformError)
	c.Assert(ok, gocheck.Equals, true)
}

func (s *PlatformSuite) TestPlatformAddWithoutName(c *gocheck.C) {
	err := PlatformAdd("", nil, nil)
	c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Platform name is required.")
}

func (s *PlatformSuite) TestPlatformUpdate(c *gocheck.C) {
    conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
    name := "test_platform_update"
	args := make(map[string]string)
	args["dockerfile"] = "http://localhost/Dockerfile"
	err = PlatformUpdate(name, args, nil)
    c.Assert(err, gocheck.NotNil)
	c.Assert(err.Error(), gocheck.Equals, "Platform doesn't exists.")
    p := Platform{Name: name}
    err = conn.Platforms().Insert(p)
	c.Assert(err, gocheck.IsNil)
	defer conn.Platforms().Remove(bson.M{"_id": name})
	err = PlatformUpdate(name, args, nil)
    c.Assert(err, gocheck.IsNil)
}

func (s *PlatformSuite) TestPlatformUpdateWithoutName(c *gocheck.C) {
    err := PlatformUpdate("", nil, nil)
    c.Assert(err, gocheck.NotNil)
    c.Assert(err.Error(), gocheck.Equals, "Platform name is required.")
}

func (s *PlatformSuite) TestPlatformUpdateShouldSetUpdatePlatformFlagOnApps(c *gocheck.C) {
    conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
    name := "test_platform_update"
	args := make(map[string]string)
	args["dockerfile"] = "http://localhost/Dockerfile"
    p := Platform{Name: name}
    err = conn.Platforms().Insert(p)
	c.Assert(err, gocheck.IsNil)
	defer conn.Platforms().Remove(bson.M{"_id": name})
    appName := "test_app"
    app := App{
        Name: appName,
        Platform: name,
    }
    err = conn.Apps().Insert(app)
	c.Assert(err, gocheck.IsNil)
    defer conn.Apps().Remove(bson.M{"_id": appName})
	err = PlatformUpdate(name, args, nil)
    a, err := GetByName(appName)
    c.Assert(err, gocheck.IsNil)
    c.Assert(a.UpdatePlatform, gocheck.Equals, true)
}
