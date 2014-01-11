// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package db encapsulates tsuru connection with MongoDB.
//
// The function Open dials to MongoDB and returns a connection (represented by
// the storage.Storage type). It manages an internal pool of connections, and
// reconnects in case of failures. That means that you should not store
// references to the connection, but always call Open.
package db

import (
	"github.com/globocom/tsuru/db/storage"
	"github.com/globocom/config"
	"labix.org/v2/mgo"
)

const (
	DefaultDatabaseURL  = "127.0.0.1:27017"
	DefaultDatabaseName = "tsuru"
)

type TsrStorage struct {
    storage *storage.Storage
}

// Conn reads the tsuru config and calls storage.Open to get a database connection.
//
// Most tsuru packages should probably use this function. storage.Open is intended for
// use when supporting more than one database.
func Conn() (*storage.Storage, error) {
	url, _ := config.GetString("database:url")
	if url == "" {
		url = DefaultDatabaseURL
	}
	dbname, _ := config.GetString("database:name")
	if dbname == "" {
		dbname = DefaultDatabaseName
	}
	return storage.Open(url, dbname)
}

func NewStorage() (*TsrStorage, error) {
    strg := &TsrStorage{}
    var err error
    strg.storage, err = Conn()
    return strg, err
}

func (s *TsrStorage) Close() {
    s.storage.Close()
}

func (s *TsrStorage) Collection(c string) *storage.Collection {
    return s.storage.Collection(c)
}

// Apps returns the apps collection from MongoDB.
func (s *TsrStorage) Apps() *storage.Collection {
	nameIndex := mgo.Index{Key: []string{"name"}, Unique: true}
	c := s.storage.Collection("apps")
	c.EnsureIndex(nameIndex)
	return c
}

func (s *TsrStorage) Deploys() *storage.Collection {
	return s.storage.Collection("deploys")
}

// Platforms returns the platforms collection from MongoDB.
func (s *TsrStorage) Platforms() *storage.Collection {
	return s.storage.Collection("platforms")
}

// Logs returns the logs collection from MongoDB.
func (s *TsrStorage) Logs() *storage.Collection {
	appNameIndex := mgo.Index{Key: []string{"appname"}}
	sourceIndex := mgo.Index{Key: []string{"source"}}
	dateAscIndex := mgo.Index{Key: []string{"date"}}
	dateDescIndex := mgo.Index{Key: []string{"-date"}}
	c := s.storage.Collection("logs")
	c.EnsureIndex(appNameIndex)
	c.EnsureIndex(sourceIndex)
	c.EnsureIndex(dateAscIndex)
	c.EnsureIndex(dateDescIndex)
	return c
}

// Services returns the services collection from MongoDB.
func (s *TsrStorage) Services() *storage.Collection {
	return s.storage.Collection("services")
}

// ServiceInstances returns the services_instances collection from MongoDB.
func (s *TsrStorage) ServiceInstances() *storage.Collection {
	return s.storage.Collection("service_instances")
}

// Users returns the users collection from MongoDB.
func (s *TsrStorage) Users() *storage.Collection {
	emailIndex := mgo.Index{Key: []string{"email"}, Unique: true}
    c := s.storage.Collection("users")
	c.EnsureIndex(emailIndex)
	return c
}

func (s *TsrStorage) Tokens() *storage.Collection {
	return s.storage.Collection("tokens")
}

func (s *TsrStorage) PasswordTokens() *storage.Collection {
	return s.storage.Collection("password_tokens")
}

func (s *TsrStorage) UserActions() *storage.Collection {
	return s.storage.Collection("user_actions")
}

// Teams returns the teams collection from MongoDB.
func (s *TsrStorage) Teams() *storage.Collection {
	return s.storage.Collection("teams")
}

// Quota returns the quota collection from MongoDB.
func (s *TsrStorage) Quota() *storage.Collection {
	userIndex := mgo.Index{Key: []string{"owner"}, Unique: true}
	c := s.storage.Collection("quota")
	c.EnsureIndex(userIndex)
	return c
}
