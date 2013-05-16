// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/globocom/config"
	"github.com/globocom/tsuru/app"
	"github.com/globocom/tsuru/db"
	"github.com/globocom/tsuru/exec"
	"github.com/globocom/tsuru/log"
	"github.com/globocom/tsuru/provision"
	"github.com/globocom/tsuru/queue"
	"github.com/globocom/tsuru/router"
	_ "github.com/globocom/tsuru/router/hipache"
	_ "github.com/globocom/tsuru/router/nginx"
	_ "github.com/globocom/tsuru/router/testing"
	"io"
	"io/ioutil"
	"labix.org/v2/mgo"
	"net"
	"strings"
	"sync"
)

func init() {
	provision.Register("docker", &dockerProvisioner{})
}

var (
	execut exec.Executor
	emutex sync.Mutex
)

func executor() exec.Executor {
	emutex.Lock()
	defer emutex.Unlock()
	if execut == nil {
		execut = exec.OsExecutor{}
	}
	return execut
}

func getRouter() (router.Router, error) {
	r, err := config.GetString("docker:router")
	if err != nil {
		return nil, err
	}
	return router.Get(r)
}

type dockerProvisioner struct{}

// Provision creates a route for the container
func (p *dockerProvisioner) Provision(app provision.App) error {
	r, err := getRouter()
	if err != nil {
		log.Printf("Failed to get router: %s", err.Error())
		return err
	}
	return r.AddBackend(app.GetName())
}

func (p *dockerProvisioner) Restart(app provision.App) error {
	containers, err := listAppContainers(app.GetName())
	if err != nil {
		log.Printf("Got error while getting app containers: %s", err)
		return err
	}
	var buf bytes.Buffer
	for _, c := range containers {
		err = c.ssh(&buf, &buf, "/var/lib/tsuru/restart")
		if err != nil {
			log.Printf("Failed to restart %q: %s.", app.GetName(), err)
			return err
		}
		buf.Reset()
	}
	return nil
}

func (p *dockerProvisioner) Deploy(a provision.App, w io.Writer) error {
	var deploy = func() error {
		c, err := newContainer(a)
		if err != nil {
			return err
		}
		err = c.deploy(w)
		if err != nil {
			c.remove()
		}
		return err
	}
	if containers, err := listAppContainers(a.GetName()); err == nil && len(containers) > 0 {
		for _, c := range containers {
			err = deploy()
			if err != nil {
				return err
			}
			a.RemoveUnit(c.Id)
		}
	} else if err := deploy(); err != nil {
		return err
	}
	app.Enqueue(queue.Message{
		Action: app.RegenerateApprcAndStart,
		Args:   []string{a.GetName()},
	})
	return nil
}

func (p *dockerProvisioner) Destroy(app provision.App) error {
	containers, _ := listAppContainers(app.GetName())
	for _, c := range containers {
		go func(c container) {
			c.remove()
		}(c)
	}
	r, err := getRouter()
	if err != nil {
		log.Printf("Failed to get router: %s", err.Error())
		return err
	}
	return r.RemoveBackend(app.GetName())
}

func (*dockerProvisioner) Addr(app provision.App) (string, error) {
	r, err := getRouter()
	if err != nil {
		log.Printf("Failed to get router: %s", err.Error())
		return "", err
	}
	addr, err := r.Addr(app.GetName())
	if err != nil {
		log.Printf("Failed to obtain app %s address: %s", app.GetName(), err.Error())
		return "", err
	}
	return addr, nil
}

func (*dockerProvisioner) AddUnits(a provision.App, units uint) ([]provision.Unit, error) {
	if units == 0 {
		return nil, errors.New("Cannot add 0 units")
	}
	containers, err := listAppContainers(a.GetName())
	if err != nil {
		return nil, err
	}
	if len(containers) < 1 {
		return nil, errors.New("New units can only be added after the first deployment")
	}
	writer := app.LogWriter{App: a, Writer: ioutil.Discard}
	result := make([]provision.Unit, int(units))
	for i := uint(0); i < units; i++ {
		container, err := newContainer(a)
		if err != nil {
			return nil, err
		}
		go container.deploy(&writer)
		result[i] = provision.Unit{
			Name:    container.Id,
			AppName: a.GetName(),
			Type:    a.GetPlatform(),
			Ip:      container.Ip,
			Status:  provision.StatusInstalling,
		}
	}
	return result, nil
}

func (*dockerProvisioner) RemoveUnit(app provision.App, unitName string) error {
	container, err := getContainer(unitName)
	if err != nil {
		return err
	}
	if container.AppName != app.GetName() {
		return errors.New("Unit does not belong to this app")
	}
	return container.remove()
}

func (*dockerProvisioner) InstallDeps(app provision.App, w io.Writer) error {
	return nil
}

func (*dockerProvisioner) ExecuteCommand(stdout, stderr io.Writer, app provision.App, cmd string, args ...string) error {
	containers, err := listAppContainers(app.GetName())
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return errors.New("No containers for this app")
	}
	for _, c := range containers {
		err = c.ssh(stdout, stderr, cmd, args...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *dockerProvisioner) CollectStatus() ([]provision.Unit, error) {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return nil, err
	}
	out, err := runCmd(docker, "ps", "-q")
	if err != nil {
		return nil, err
	}
	var linesGroup sync.WaitGroup
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	units := make(chan provision.Unit, len(lines))
	result := buildResult(len(lines), units)
	errs := make(chan error, 1)
	for _, line := range lines {
		linesGroup.Add(1)
		go collectUnit(line, units, errs, &linesGroup)
	}
	linesGroup.Wait()
	close(errs)
	close(units)
	if err, ok := <-errs; ok {
		return nil, err
	}
	return <-result, nil
}

func collectUnit(id string, units chan<- provision.Unit, errs chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	docker, _ := config.GetString("docker:binary")
	container, err := getContainer(id)
	if err != nil {
		log.Printf("Container %q not in the database. Skipping...", id)
		return
	}
	unit := provision.Unit{
		Name:    container.Id,
		AppName: container.AppName,
		Type:    container.Type,
	}
	switch container.Status {
	case "error":
		unit.Status = provision.StatusError
		units <- unit
		fallthrough
	case "created":
		return
	}
	out, err := runCmd(docker, "inspect", id)
	if err != nil {
		errs <- err
		return
	}
	var c map[string]interface{}
	err = json.Unmarshal([]byte(out), &c)
	if err != nil {
		errs <- err
		return
	}
	unit.Ip = c["NetworkSettings"].(map[string]interface{})["IpAddress"].(string)
	addr := fmt.Sprintf("%s:%s", unit.Ip, container.Port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		unit.Status = provision.StatusInstalling
	} else {
		conn.Close()
		unit.Status = provision.StatusStarted
	}
	units <- unit
}

func buildResult(maxSize int, units <-chan provision.Unit) <-chan []provision.Unit {
	ch := make(chan []provision.Unit, 1)
	go func() {
		result := make([]provision.Unit, 0, maxSize)
		for unit := range units {
			result = append(result, unit)
		}
		ch <- result
	}()
	return ch
}

func collection() *mgo.Collection {
	name, err := config.GetString("docker:collection")
	if err != nil {
		log.Fatalf("FATAL: %s.", err)
	}
	conn, err := db.Conn()
	if err != nil {
		log.Printf("Failed to connect to the database: %s", err)
	}
	return conn.Collection(name)
}

func imagesCollection() *mgo.Collection {
	nameIndex := mgo.Index{Key: []string{"name"}, Unique: true}
	conn, err := db.Conn()
	if err != nil {
		log.Printf("Failed to connect to the database: %s", err)
	}
	c := conn.Collection("docker_image")
	c.EnsureIndex(nameIndex)
	return c
}
