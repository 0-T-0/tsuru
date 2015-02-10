// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"sync"

	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/log"
	_ "github.com/tsuru/tsuru/router/hipache"
	_ "github.com/tsuru/tsuru/router/routertest"
	"gopkg.in/mgo.v2/bson"
)

func (p *dockerProvisioner) fixContainers() error {
	var containersGroup sync.WaitGroup
	containers, err := p.listAllContainers()
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return nil
	}
	for _, container := range containers {
		containersGroup.Add(1)
		go p.checkContainer(container, &containersGroup)
	}
	containersGroup.Wait()
	return nil
}

func (p *dockerProvisioner) checkContainer(container container, wg *sync.WaitGroup) error {
	if wg != nil {
		defer wg.Done()
	}
	if container.available() {
		info, err := container.networkInfo(p)
		if err != nil {
			return err
		}
		if info.HTTPHostPort != container.HostPort || info.IP != container.IP {
			err = p.fixContainer(&container, info)
			if err != nil {
				log.Errorf("error on fix container hostport for [container %s]", container.ID)
				return err
			}
		}
	}
	return nil
}

func (p *dockerProvisioner) fixContainer(container *container, info containerNetworkInfo) error {
	if info.HTTPHostPort == "" {
		return nil
	}
	appInstance, err := app.GetByName(container.AppName)
	if err != nil {
		return err
	}
	router, err := getRouterForApp(appInstance)
	if err != nil {
		return err
	}
	router.RemoveRoute(container.AppName, container.getAddress())
	container.IP = info.IP
	container.HostPort = info.HTTPHostPort
	router.AddRoute(container.AppName, container.getAddress())
	coll := p.collection()
	defer coll.Close()
	return coll.Update(bson.M{"id": container.ID}, container)
}
