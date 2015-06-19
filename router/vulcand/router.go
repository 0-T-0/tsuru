// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulcand

import (
	"crypto/md5"
	"fmt"

	"github.com/tsuru/config"
	"github.com/tsuru/tsuru/router"

	vulcandAPI "github.com/mailgun/vulcand/api"
	vulcandEng "github.com/mailgun/vulcand/engine"
	vulcandReg "github.com/mailgun/vulcand/plugin/registry"
)

const routerName = "vulcand"

func init() {
	router.Register(routerName, createRouter)
}

type vulcandRouter struct {
	client *vulcandAPI.Client
	prefix string
	domain string
}

func createRouter(prefix string) (router.Router, error) {
	vURL, err := config.GetString(prefix + ":api-url")
	if err != nil {
		return nil, err
	}

	domain, err := config.GetString(prefix + ":domain")
	if err != nil {
		return nil, err
	}

	client := vulcandAPI.NewClient(vURL, vulcandReg.GetRegistry())
	vRouter := &vulcandRouter{
		client: client,
		prefix: prefix,
		domain: domain,
	}

	return vRouter, nil
}

func (r *vulcandRouter) frontendHostname(app string) string {
	return fmt.Sprintf("%s.%s", app, r.domain)
}

func (r *vulcandRouter) frontendName(hostname string) string {
	return fmt.Sprintf("tsuru_%s", hostname)
}

func (r *vulcandRouter) backendName(app string) string {
	return fmt.Sprintf("tsuru_%s", app)
}

func (r *vulcandRouter) serverName(address string) string {
	return fmt.Sprintf("tsuru_%x", md5.Sum([]byte(address)))
}

func (r *vulcandRouter) AddBackend(name string) error {
	backendName := r.backendName(name)
	frontendName := r.frontendName(r.frontendHostname(name))
	backendKey := vulcandEng.BackendKey{Id: backendName}
	frontendKey := vulcandEng.FrontendKey{Id: frontendName}

	if found, _ := r.client.GetBackend(backendKey); found != nil {
		return router.ErrBackendExists
	}
	if found, _ := r.client.GetFrontend(frontendKey); found != nil {
		return router.ErrBackendExists
	}

	backend, err := vulcandEng.NewHTTPBackend(
		backendName,
		vulcandEng.HTTPBackendSettings{},
	)
	if err != nil {
		return err
	}
	err = r.client.UpsertBackend(*backend)
	if err != nil {
		return err
	}

	frontend, err := vulcandEng.NewHTTPFrontend(
		frontendName,
		backend.Id,
		fmt.Sprintf(`Host(%q) && PathRegexp("/")`, r.frontendHostname(name)),
		vulcandEng.HTTPFrontendSettings{},
	)
	if err != nil {
		return err
	}

	err = r.client.UpsertFrontend(*frontend, vulcandEng.NoTTL)
	if err != nil {
		return err
	}

	return router.Store(name, name, routerName)
}

func (r *vulcandRouter) RemoveBackend(name string) error {
	frontendKey := vulcandEng.FrontendKey{Id: r.frontendName(r.frontendHostname(name))}
	err := r.client.DeleteFrontend(frontendKey)
	if err != nil {
		if _, ok := err.(*vulcandEng.NotFoundError); ok {
			return router.ErrBackendNotFound
		}
		return err
	}

	backendKey := vulcandEng.BackendKey{Id: r.backendName(name)}
	err = r.client.DeleteBackend(backendKey)
	if err != nil {
		return err
	}

	return router.Remove(name)
}

func (r *vulcandRouter) AddRoute(name, address string) error {
	serverKey := vulcandEng.ServerKey{
		Id:         r.serverName(address),
		BackendKey: vulcandEng.BackendKey{Id: r.backendName(name)},
	}

	if found, _ := r.client.GetServer(serverKey); found != nil {
		return router.ErrRouteExists
	}

	server, err := vulcandEng.NewServer(serverKey.Id, address)
	if err != nil {
		return err
	}

	return r.client.UpsertServer(serverKey.BackendKey, *server, vulcandEng.NoTTL)
}

func (r *vulcandRouter) RemoveRoute(name, address string) error {
	serverKey := vulcandEng.ServerKey{
		Id:         r.serverName(address),
		BackendKey: vulcandEng.BackendKey{Id: r.backendName(name)},
	}
	err := r.client.DeleteServer(serverKey)
	if err != nil {
		if _, ok := err.(*vulcandEng.NotFoundError); ok {
			return router.ErrRouteNotFound
		}
	}
	return err
}

func (r *vulcandRouter) SetCName(cname, name string) error {
	frontendName := r.frontendName(cname)
	if found, _ := r.client.GetFrontend(vulcandEng.FrontendKey{Id: frontendName}); found != nil {
		return router.ErrRouteExists
	}

	frontend, err := vulcandEng.NewHTTPFrontend(
		frontendName,
		r.backendName(name),
		fmt.Sprintf(`Host(%q) && PathRegexp("/")`, cname),
		vulcandEng.HTTPFrontendSettings{},
	)
	if err != nil {
		return err
	}
	return r.client.UpsertFrontend(*frontend, vulcandEng.NoTTL)
}

func (r *vulcandRouter) UnsetCName(cname, name string) error {
	frontendKey := vulcandEng.FrontendKey{Id: r.frontendName(cname)}
	err := r.client.DeleteFrontend(frontendKey)
	if err != nil {
		if _, ok := err.(*vulcandEng.NotFoundError); ok {
			return router.ErrRouteNotFound
		}
	}
	return err
}

func (r *vulcandRouter) Addr(name string) (string, error) {
	frontendHostname := r.frontendHostname(name)
	frontendKey := vulcandEng.FrontendKey{Id: r.frontendName(frontendHostname)}
	if found, _ := r.client.GetFrontend(frontendKey); found == nil {
		return "", router.ErrRouteNotFound
	}
	return frontendHostname, nil
}

func (r *vulcandRouter) Swap(backend1, backend2 string) error {
	return router.Swap(r, backend1, backend2)
}

func (r *vulcandRouter) Routes(name string) ([]string, error) {
	servers, err := r.client.GetServers(vulcandEng.BackendKey{
		Id: r.backendName(name),
	})
	if err != nil {
		return []string{}, err
	}

	routes := make([]string, len(servers))
	for i, server := range servers {
		routes[i] = server.URL
	}
	return routes, nil
}

func (r *vulcandRouter) StartupMessage() (string, error) {
	message := fmt.Sprintf("vulcand router %q with API at %q", r.domain, r.client.Addr)
	return message, nil
}
