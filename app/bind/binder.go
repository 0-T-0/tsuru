// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bind provides interfaces and types for use when binding an app to a
// service.
package bind

import "io"

// EnvVar represents a environment variable for an app.
type EnvVar struct {
	Name         string
	Value        string
	Public       bool
	InstanceName string
}

type Unit interface {
	// GetIp returns the unit ip.
	GetIp() string
}

type App interface {
	// GetIp returns the app ip.
	GetIp() string

	// GetName returns the app name.
	GetName() string

	// GetUnits returns the app units.
	GetUnits() []Unit

	// InstanceEnv returns the app enviroment variables.
	InstanceEnv(string) map[string]EnvVar

	// GetEnv returns the given env var, or an error when it's not defined.
	GetEnv(name string) (EnvVar, error)

	// SetEnvs adds enviroment variables in the app.
	SetEnvs(envs []EnvVar, publicOnly bool, w io.Writer) error

	// UnsetEnvs removes the given enviroment variables from the app.
	UnsetEnvs(envNames []string, publicOnly bool, w io.Writer) error
}

type Binder interface {
	// BindApp makes the bind between the binder and an app.
	BindApp(App) error

	// BindUnit makes the bind between the binder and an unit.
	BindUnit(App, Unit) error

	// UnbindApp makes the unbind between the binder and an app.
	UnbindApp(App) error

	// UnbindUnit makes the unbind between the binder and an unit.
	UnbindUnit(App, Unit) error
}
