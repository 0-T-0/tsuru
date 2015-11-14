// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/tsuru/tsuru/auth"
	"github.com/tsuru/tsuru/db"
	tsuruIo "github.com/tsuru/tsuru/io"
	"github.com/tsuru/tsuru/log"
	"github.com/tsuru/tsuru/provision"
	"github.com/tsuru/tsuru/repository"
	"github.com/tsuru/tsuru/service"
	"gopkg.in/mgo.v2/bson"
)

type DeployData struct {
	ID          bson.ObjectId `bson:"_id,omitempty"`
	App         string
	Timestamp   time.Time
	Duration    time.Duration
	Commit      string
	Error       string
	Image       string
	Log         string
	User        string
	Origin      string
	CanRollback bool
	RemoveDate  time.Time `bson:",omitempty"`
}

type DiffDeployData struct {
	DeployData
	Diff string
}

func (d *DiffDeployData) MarshalJSON() ([]byte, error) {
	var err error
	if d.Diff == "" {
		d.Diff, err = GetDiffInDeploys(&d.DeployData)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(*d)
}

func (app *App) ListDeploys(u *auth.User) ([]DeployData, error) {
	return listDeploys(app, nil, u, 0, 0)
}

// ListDeploys returns the list of deploy that the given user has access to.
//
// If the user does not have acces to any deploy, this function returns an empty
// list and a nil error.
//
// The deploy list can be filtered by app or service.
func ListDeploys(app *App, s *service.Service, u *auth.User, skip, limit int) ([]DeployData, error) {
	return listDeploys(app, s, u, skip, limit)
}

func userHasPermission(u *auth.User, appName string) bool {
	appsByUser, err := List(u, nil)
	if err != nil {
		return false
	}
	for _, app := range appsByUser {
		if app.Name == appName {
			return true
		}
	}
	return false
}

func listDeploys(app *App, s *service.Service, u *auth.User, skip, limit int) ([]DeployData, error) {
	conn, err := db.Conn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	appsByName := set{}
	if app != nil {
		appsByName.Add(app.Name)
	}
	appsByUser := set{}
	if u != nil {
		appsList, _ := List(u, nil)
		for _, a := range appsList {
			appsByUser.Add(a.Name)
		}
	}
	appsByService := set{}
	if s != nil {
		appList := listAppsByService(s.Name)
		for _, a := range appList {
			appsByService.Add(a)
		}
	}
	appsIntersection := appsByService.Intersection(appsByUser.Intersection(appsByName))
	apps := []string{}
	for key := range appsIntersection {
		apps = append(apps, key)
	}
	var list []DeployData
	query := conn.Deploys().Find(bson.M{"app": bson.M{"$in": apps}, "removedate": bson.M{"$exists": false}}).Sort("-timestamp")
	if skip != 0 {
		query = query.Skip(skip)
	}
	if limit != 0 {
		query = query.Limit(limit)
	}
	if err = query.All(&list); err != nil {
		return nil, err
	}
	validImages := set{}
	for _, appName := range apps {
		var imgs []string
		imgs, err = Provisioner.ValidAppImages(appName)
		if err != nil {
			return nil, err
		}
		validImages.Add(imgs...)
	}
	for i := range list {
		list[i].CanRollback = validImages.Includes(list[i].Image)
		r := regexp.MustCompile("v[0-9]+$")
		if list[i].Image != "" && r.MatchString(list[i].Image) {
			parts := r.FindAllStringSubmatch(list[i].Image, -1)
			list[i].Image = parts[0][0]
		}
	}
	return list, err
}

func markDeploysAsRemoved(appName string) error {
	conn, err := db.Conn()
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Deploys().UpdateAll(
		bson.M{"app": appName, "removedate": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"removedate": time.Now()}},
	)
	return err
}

func listAppsByService(serviceName string) []string {
	var apps []string
	var instances []service.ServiceInstance
	q := bson.M{"service_name": serviceName}
	conn, err := db.Conn()
	if err != nil {
		return nil
	}
	defer conn.Close()
	conn.ServiceInstances().Find(q).All(&instances)
	for _, instance := range instances {
		for _, app := range instance.Apps {
			apps = append(apps, app)
		}
	}
	return apps
}

func GetDeploy(id string, u *auth.User) (*DeployData, error) {
	var dep DeployData
	conn, err := db.Conn()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if !bson.IsObjectIdHex(id) {
		return nil, fmt.Errorf("id parameter is not ObjectId: %s", id)
	}
	if err := conn.Deploys().FindId(bson.ObjectIdHex(id)).One(&dep); err != nil {
		return nil, err
	}
	if userHasPermission(u, dep.App) {
		return &dep, nil
	}
	return nil, errors.New("Deploy not found.")
}

func GetDiffInDeploys(d *DeployData) (string, error) {
	var list []DeployData
	conn, err := db.Conn()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := conn.Deploys().Find(bson.M{"app": d.App, "_id": bson.M{"$lte": d.ID}}).Sort("-timestamp").Limit(2).All(&list); err != nil {
		return "", err
	}
	if len(list) < 2 {
		return "The deployment must have at least two commits for the diff.", nil
	}
	if list[0].Origin != "git" || list[1].Origin != "git" {
		return fmt.Sprintf("Cannot have diffs between %s based and %s based deployments", list[1].Origin, list[0].Origin), nil
	}
	return repository.Manager().Diff(d.App, list[1].Commit, list[0].Commit)
}

type DeployOptions struct {
	App          *App
	Version      string
	Commit       string
	ArchiveURL   string
	File         io.ReadCloser
	OutputStream io.Writer
	User         string
	Image        string
}

// Deploy runs a deployment of an application. It will first try to run an
// archive based deploy (if opts.ArchiveURL is not empty), and then fallback to
// the Git based deployment.
func Deploy(opts DeployOptions) error {
	var outBuffer bytes.Buffer
	start := time.Now()
	logWriter := LogWriter{App: opts.App}
	logWriter.Async()
	defer logWriter.Close()
	writer := io.MultiWriter(&tsuruIo.NoErrorWriter{Writer: opts.OutputStream}, &outBuffer, &logWriter)
	imageId, err := deployToProvisioner(&opts, writer)
	elapsed := time.Since(start)
	saveErr := saveDeployData(&opts, imageId, outBuffer.String(), elapsed, err)
	if saveErr != nil {
		log.Errorf("WARNING: couldn't save deploy data, deploy opts: %#v", opts)
	}
	if err != nil {
		return err
	}
	err = incrementDeploy(opts.App)
	if err != nil {
		log.Errorf("WARNING: couldn't increment deploy count, deploy opts: %#v", opts)
	}
	if opts.App.UpdatePlatform == true {
		opts.App.SetUpdatePlatform(false)
	}
	return nil
}

func deployToProvisioner(opts *DeployOptions, writer io.Writer) (string, error) {
	if opts.Image != "" {
		if deployer, ok := Provisioner.(provision.ImageDeployer); ok {
			return deployer.ImageDeploy(opts.App, opts.Image, writer)
		}
	}
	if opts.File != nil {
		if deployer, ok := Provisioner.(provision.UploadDeployer); ok {
			return deployer.UploadDeploy(opts.App, opts.File, writer)
		}
	}
	if opts.ArchiveURL != "" {
		if deployer, ok := Provisioner.(provision.ArchiveDeployer); ok {
			return deployer.ArchiveDeploy(opts.App, opts.ArchiveURL, writer)
		}
	}
	return Provisioner.(provision.GitDeployer).GitDeploy(opts.App, opts.Version, writer)
}

func saveDeployData(opts *DeployOptions, imageId, log string, duration time.Duration, deployError error) error {
	conn, err := db.Conn()
	if err != nil {
		return err
	}
	defer conn.Close()
	deploy := DeployData{
		App:       opts.App.Name,
		Timestamp: time.Now(),
		Duration:  duration,
		Commit:    opts.Commit,
		Image:     imageId,
		Log:       log,
		User:      opts.User,
	}
	if opts.Commit != "" {
		deploy.Origin = "git"
	} else if opts.Image != "" {
		deploy.Origin = "rollback"
	} else {
		deploy.Origin = "app-deploy"
	}
	if deployError != nil {
		deploy.Error = deployError.Error()
	}
	return conn.Deploys().Insert(deploy)
}

func incrementDeploy(app *App) error {
	conn, err := db.Conn()
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Apps().Update(
		bson.M{"name": app.Name},
		bson.M{"$inc": bson.M{"deploys": 1}},
	)
}

func getImage(appName string, img string) (string, error) {
	conn, err := db.Conn()
	if err != nil {
		return "", err
	}
	defer conn.Close()
	var deploy DeployData
	query := bson.M{"app": appName, "image": bson.M{"$regex": ".*:" + img + "$"}}
	if err := conn.Deploys().Find(query).One(&deploy); err != nil {
		return "", err
	}
	return deploy.Image, nil
}

func Rollback(opts DeployOptions) error {
	if !regexp.MustCompile(":v[0-9]+$").MatchString(opts.Image) {
		img, err := getImage(opts.App.Name, opts.Image)
		// err is not handled here because it is handled by Deploy
		if err == nil {
			opts.Image = img
		}
	}
	return Deploy(opts)
}
