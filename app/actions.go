// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"errors"
	"fmt"
	"github.com/globocom/config"
	"github.com/globocom/go-gandalfclient"
	"github.com/globocom/tsuru/action"
	"github.com/globocom/tsuru/app/bind"
	"github.com/globocom/tsuru/auth"
	"github.com/globocom/tsuru/db"
	"github.com/globocom/tsuru/log"
	"github.com/globocom/tsuru/quota"
	"github.com/globocom/tsuru/repository"
	"labix.org/v2/mgo/bson"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/iam"
	"strconv"
	"strings"
)

// reserveUserApp reserves the app for the user, only if the user has a quota
// of apps. If the user does not have a quota, meaning that it's unlimited,
// reserveUserApp.Forward just return nil.
var reserveUserApp = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		var app App
		switch ctx.Params[0].(type) {
		case App:
			app = ctx.Params[0].(App)
		case *App:
			app = *ctx.Params[0].(*App)
		default:
			return nil, errors.New("First parameter must be App or *App.")
		}
		var user auth.User
		switch ctx.Params[2].(type) {
		case auth.User:
			user = ctx.Params[2].(auth.User)
		case *auth.User:
			user = *ctx.Params[2].(*auth.User)
		default:
			return nil, errors.New("Third parameter must be auth.User or *auth.User.")
		}
		if err := quota.Reserve(user.Email, app.Name); err == quota.ErrQuotaExceeded {
			return nil, err
		}
		return map[string]string{"app": app.Name, "user": user.Email}, nil
	},
	Backward: func(ctx action.BWContext) {
		m := ctx.FWResult.(map[string]string)
		quota.Release(m["user"], m["app"])
	},
	MinParams: 3,
}

var createAppQuota = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		var app App
		switch ctx.Params[0].(type) {
		case App:
			app = ctx.Params[0].(App)
		case *App:
			app = *ctx.Params[0].(*App)
		default:
			return nil, errors.New("First parameter must be App or *App.")
		}
		if limit, err := config.GetUint("quota:units-per-app"); err == nil {
			if limit == 0 {
				return nil, errors.New("app creation is disallowed")
			}
			quota.Create(app.Name, uint(limit))
		}
		return app.Name, nil
	},
	Backward: func(ctx action.BWContext) {
		quota.Delete(ctx.FWResult.(string))
	},
	MinParams: 1,
}

// insertApp is an action that inserts an app in the database in Forward and
// removes it in the Backward.
//
// The first argument in the context must be an App or a pointer to an App.
var insertApp = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		var app App
		switch ctx.Params[0].(type) {
		case App:
			app = ctx.Params[0].(App)
		case *App:
			app = *ctx.Params[0].(*App)
		default:
			return nil, errors.New("First parameter must be App or *App.")
		}
		conn, err := db.Conn()
		if err != nil {
			return nil, err
		}
		defer conn.Close()
		err = conn.Apps().Insert(app)
		if err != nil && strings.HasPrefix(err.Error(), "E11000") {
			return nil, errors.New("there is already an app with this name.")
		}
		return &app, err
	},
	Backward: func(ctx action.BWContext) {
		app := ctx.FWResult.(*App)
		conn, err := db.Conn()
		if err != nil {
			log.Printf("Could not connect to the database: %s", err)
			return
		}
		defer conn.Close()
		conn.Apps().Remove(bson.M{"name": app.Name})
	},
	MinParams: 1,
}

type createBucketResult struct {
	app *App
	env *s3Env
}

// createIAMUserAction creates a user in IAM. It requires that the first
// parameter is the a pointer to an App instance.
var createIAMUserAction = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		app := ctx.Previous.(*App)
		return createIAMUser(app.Name)
	},
	Backward: func(ctx action.BWContext) {
		user := ctx.FWResult.(*iam.User)
		getIAMEndpoint().DeleteUser(user.Name)
	},
	MinParams: 1,
}

// createIAMAccessKeyAction creates an access key in IAM. It uses the result
// returned by createIAMUserAction, so it must come after this action.
var createIAMAccessKeyAction = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		user := ctx.Previous.(*iam.User)
		return createIAMAccessKey(user)
	},
	Backward: func(ctx action.BWContext) {
		key := ctx.FWResult.(*iam.AccessKey)
		getIAMEndpoint().DeleteAccessKey(key.Id, key.UserName)
	},
	MinParams: 1,
}

// createBucketAction creates a bucket in S3. It uses the result of
// createIAMAccessKeyAction for managing permission, and the app given as
// parameter to generate the name of the bucket. It must run after
// createIAMAccessKey.
var createBucketAction = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		app := ctx.Params[0].(*App)
		key := ctx.Previous.(*iam.AccessKey)
		bucket, err := putBucket(app.Name)
		if err != nil {
			return nil, err
		}
		env := s3Env{
			Auth: aws.Auth{
				AccessKey: key.Id,
				SecretKey: key.Secret,
			},
			bucket:             bucket.Name,
			endpoint:           bucket.S3Endpoint,
			locationConstraint: bucket.S3LocationConstraint,
		}
		return &env, nil
	},
	Backward: func(ctx action.BWContext) {
		env := ctx.FWResult.(*s3Env)
		getS3Endpoint().Bucket(env.bucket).DelBucket()
	},
	MinParams: 1,
}

// createUserPolicyAction creates a new UserPolicy in IAM. It requires a
// pointer to an App instance as the first parameter, and the previous result
// to be a *s3Env (it should be used after createBucketAction).
var createUserPolicyAction = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		app := ctx.Params[0].(*App)
		env := ctx.Previous.(*s3Env)
		_, err := createIAMUserPolicy(&iam.User{Name: app.Name}, app.Name, env.bucket)
		if err != nil {
			return nil, err
		}
		return ctx.Previous, nil
	},
	Backward: func(ctx action.BWContext) {
		app := ctx.Params[0].(*App)
		policyName := fmt.Sprintf("app-%s-bucket", app.Name)
		getIAMEndpoint().DeleteUserPolicy(app.Name, policyName)
	},
	MinParams: 1,
}

// exportEnvironmentsAction exports tsuru's default environment variables in a
// new app. It requires a pointer to an App instance as the first parameter,
// and the previous result to be a *s3Env (it should be used after
// createUserPolicyAction or createBucketAction).
var exportEnvironmentsAction = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		app := ctx.Params[0].(*App)
		err := app.Get()
		if err != nil {
			return nil, err
		}
		t, err := auth.CreateApplicationToken(app.Name)
		if err != nil {
			return nil, err
		}
		host, _ := config.GetString("host")
		envVars := []bind.EnvVar{
			{Name: "TSURU_APPNAME", Value: app.Name},
			{Name: "TSURU_HOST", Value: host},
			{Name: "TSURU_APP_TOKEN", Value: t.Token},
		}
		env, ok := ctx.Previous.(*s3Env)
		if ok {
			variables := map[string]string{
				"ENDPOINT":           env.endpoint,
				"LOCATIONCONSTRAINT": strconv.FormatBool(env.locationConstraint),
				"ACCESS_KEY_ID":      env.AccessKey,
				"SECRET_KEY":         env.SecretKey,
				"BUCKET":             env.bucket,
			}
			for name, value := range variables {
				envVars = append(envVars, bind.EnvVar{
					Name:         fmt.Sprintf("TSURU_S3_%s", name),
					Value:        value,
					InstanceName: s3InstanceName,
				})
			}
		}
		err = app.setEnvsToApp(envVars, false, true)
		if err != nil {
			return nil, err
		}
		return ctx.Previous, nil
	},
	Backward: func(ctx action.BWContext) {
		app := ctx.Params[0].(*App)
		auth.DeleteToken(app.Env["TSURU_APP_TOKEN"].Value)
		if app.Get() == nil {
			s3Env := app.InstanceEnv(s3InstanceName)
			vars := make([]string, len(s3Env)+3)
			i := 0
			for k := range s3Env {
				vars[i] = k
				i++
			}
			vars[i] = "TSURU_HOST"
			vars[i+1] = "TSURU_APPNAME"
			vars[i+2] = "TSURU_APP_TOKEN"
			app.UnsetEnvs(vars, false)
		}
	},
	MinParams: 1,
}

// createRepository creates a repository for the app in Gandalf.
var createRepository = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		var app App
		switch ctx.Params[0].(type) {
		case App:
			app = ctx.Params[0].(App)
		case *App:
			app = *ctx.Params[0].(*App)
		default:
			return nil, errors.New("First parameter must be App or *App.")
		}
		gUrl := repository.GitServerUri()
		var users []string
		for _, t := range app.GetTeams() {
			users = append(users, t.Users...)
		}
		c := gandalf.Client{Endpoint: gUrl}
		_, err := c.NewRepository(app.Name, users, false)
		return &app, err
	},
	Backward: func(ctx action.BWContext) {
		app := ctx.FWResult.(*App)
		app.Get()
		gUrl := repository.GitServerUri()
		c := gandalf.Client{Endpoint: gUrl}
		c.RemoveRepository(app.Name)
	},
	MinParams: 1,
}

// provisionApp provisions the app in the provisioner. It takes two arguments:
// the app, and the number of units to create (an unsigned integer).
var provisionApp = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		var app App
		switch ctx.Params[0].(type) {
		case App:
			app = ctx.Params[0].(App)
		case *App:
			app = *ctx.Params[0].(*App)
		default:
			return nil, errors.New("First parameter must be App or *App.")
		}
		err := Provisioner.Provision(&app)
		if err != nil {
			return nil, err
		}
		return &app, nil
	},
	Backward: func(ctx action.BWContext) {
		app := ctx.FWResult.(*App)
		Provisioner.Destroy(app)
	},
	MinParams: 2,
}

// provisionAddUnits adds n-1 units to the app. It receives two arguments: the
// app and the total number of the units that the app must have. It assumes
// that the app already have one unit, so it adds n-1 units to the app.
//
// It reads the app from the Previos result in the context, so this action
// cannot be the first in a pipeline.
var provisionAddUnits = action.Action{
	Forward: func(ctx action.FWContext) (action.Result, error) {
		app := ctx.Previous.(*App)
		var units uint
		switch ctx.Params[1].(type) {
		case int:
			units = uint(ctx.Params[1].(int))
		case int64:
			units = uint(ctx.Params[1].(int64))
		case uint:
			units = ctx.Params[1].(uint)
		case uint64:
			units = uint(ctx.Params[1].(uint64))
		default:
			units = 1
		}
		if units > 1 {
			return nil, app.AddUnits(units - 1)
		}
		return nil, nil
	},
	MinParams: 2,
}
