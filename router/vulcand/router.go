// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulcand

import (
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

func (r *vulcandRouter) frontendName(app string) string {
	return fmt.Sprintf("tsuru_%s", r.frontendHostname(app))
}

func (r *vulcandRouter) backendName(app string) string {
	return fmt.Sprintf("tsuru_%s", app)
}

func (r *vulcandRouter) AddBackend(name string) error {
	backend, err := vulcandEng.NewHTTPBackend(
		r.backendName(name),
		vulcandEng.HTTPBackendSettings{},
	)
	if err != nil {
		return err
	}
	err = r.client.UpsertBackend(*backend)
	if err != nil {
		return err
	}

	hostname := r.frontendHostname(name)
	frontend, err := vulcandEng.NewHTTPFrontend(
		r.frontendName(name),
		backend.Id,
		fmt.Sprintf(`Host(%q) && PathRegexp("/")`, hostname),
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
	frontendKey := vulcandEng.FrontendKey{Id: r.frontendName(name)}
	err := r.client.DeleteFrontend(frontendKey)
	if err != nil {
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
	return nil
}

func (r *vulcandRouter) RemoveRoute(name, address string) error {
	return nil
}

func (r *vulcandRouter) SetCName(cname, name string) error {
	return nil
}

func (r *vulcandRouter) UnsetCName(cname, name string) error {
	return nil
}

func (r *vulcandRouter) Addr(name string) (string, error) {
	return "", nil
}

func (r *vulcandRouter) Swap(string, string) error {
	return nil
}

func (r *vulcandRouter) Routes(name string) ([]string, error) {
	return []string{}, nil
}

func (r *vulcandRouter) StartupMessage() (string, error) {
	return "", nil
}
