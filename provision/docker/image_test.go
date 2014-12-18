// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"net/http"
	"sort"

	"github.com/fsouza/go-dockerclient"
	"github.com/fsouza/go-dockerclient/testing"
	"github.com/tsuru/config"
	"github.com/tsuru/docker-cluster/cluster"
	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/db"
	"gopkg.in/mgo.v2/bson"
	"launchpad.net/gocheck"
)

func (s *S) TestMigrateImages(c *gocheck.C) {
	var requests []*http.Request
	server, err := testing.NewServer("127.0.0.1:0", nil, func(r *http.Request) {
		requests = append(requests, r)
	})
	c.Assert(err, gocheck.IsNil)
	defer server.Stop()
	cmutex.Lock()
	oldDockerCluster := dCluster
	dCluster, _ = cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: server.URL()})
	cmutex.Unlock()
	defer func() {
		cmutex.Lock()
		defer cmutex.Unlock()
		dCluster = oldDockerCluster
	}()
	app1 := app.App{Name: "app1"}
	app2 := app.App{Name: "app2"}
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	conn.Apps().Insert(app1, app2)
	defer conn.Apps().RemoveAll(bson.M{"name": bson.M{"$in": []string{app1.Name, app2.Name}}})
	err = newImage("tsuru/app1", "")
	c.Assert(err, gocheck.IsNil)
	err = newImage("tsuru/app2", "")
	c.Assert(err, gocheck.IsNil)
	err = migrateImages()
	c.Assert(err, gocheck.IsNil)
	client, err := docker.NewClient(server.URL())
	c.Assert(err, gocheck.IsNil)
	images, err := client.ListImages(docker.ListImagesOptions{All: true})
	c.Assert(err, gocheck.IsNil)
	c.Assert(images, gocheck.HasLen, 2)
	tags1 := images[0].RepoTags
	sort.Strings(tags1)
	tags2 := images[1].RepoTags
	sort.Strings(tags2)
	c.Assert(tags1, gocheck.DeepEquals, []string{"tsuru/app-app1", "tsuru/app1"})
	c.Assert(tags2, gocheck.DeepEquals, []string{"tsuru/app-app2", "tsuru/app2"})
}

func (s *S) TestMigrateImagesWithoutImageInStorage(c *gocheck.C) {
	var requests []*http.Request
	server, err := testing.NewServer("127.0.0.1:0", nil, func(r *http.Request) {
		requests = append(requests, r)
	})
	c.Assert(err, gocheck.IsNil)
	defer server.Stop()
	cmutex.Lock()
	oldDockerCluster := dCluster
	dCluster, _ = cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: server.URL()})
	cmutex.Unlock()
	defer func() {
		cmutex.Lock()
		defer cmutex.Unlock()
		dCluster = oldDockerCluster
	}()
	app1 := app.App{Name: "app1"}
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	conn.Apps().Insert(app1)
	defer conn.Apps().RemoveAll(bson.M{"name": bson.M{"$in": []string{app1.Name}}})
	err = migrateImages()
	c.Assert(err, gocheck.IsNil)
	client, err := docker.NewClient(server.URL())
	c.Assert(err, gocheck.IsNil)
	images, err := client.ListImages(docker.ListImagesOptions{All: true})
	c.Assert(err, gocheck.IsNil)
	c.Assert(images, gocheck.HasLen, 0)
}

func (s *S) TestMigrateImagesWithRegistry(c *gocheck.C) {
	var requests []*http.Request
	server, err := testing.NewServer("127.0.0.1:0", nil, func(r *http.Request) {
		requests = append(requests, r)
	})
	c.Assert(err, gocheck.IsNil)
	defer server.Stop()
	config.Set("docker:registry", "localhost:3030")
	defer config.Unset("docker:registry")
	cmutex.Lock()
	oldDockerCluster := dCluster
	dCluster, _ = cluster.New(nil, &cluster.MapStorage{},
		cluster.Node{Address: server.URL()})
	cmutex.Unlock()
	defer func() {
		cmutex.Lock()
		defer cmutex.Unlock()
		dCluster = oldDockerCluster
	}()
	app1 := app.App{Name: "app1"}
	app2 := app.App{Name: "app2"}
	conn, err := db.Conn()
	c.Assert(err, gocheck.IsNil)
	defer conn.Close()
	conn.Apps().Insert(app1, app2)
	defer conn.Apps().RemoveAll(bson.M{"name": bson.M{"$in": []string{app1.Name, app2.Name}}})
	err = newImage("localhost:3030/tsuru/app1", "")
	c.Assert(err, gocheck.IsNil)
	err = newImage("localhost:3030/tsuru/app2", "")
	c.Assert(err, gocheck.IsNil)
	err = migrateImages()
	c.Assert(err, gocheck.IsNil)
	client, err := docker.NewClient(server.URL())
	c.Assert(err, gocheck.IsNil)
	images, err := client.ListImages(docker.ListImagesOptions{All: true})
	c.Assert(err, gocheck.IsNil)
	c.Assert(images, gocheck.HasLen, 2)
	tags1 := images[0].RepoTags
	sort.Strings(tags1)
	tags2 := images[1].RepoTags
	sort.Strings(tags2)
	c.Assert(tags1, gocheck.DeepEquals, []string{"localhost:3030/tsuru/app-app1", "localhost:3030/tsuru/app1"})
	c.Assert(tags2, gocheck.DeepEquals, []string{"localhost:3030/tsuru/app-app2", "localhost:3030/tsuru/app2"})
}
