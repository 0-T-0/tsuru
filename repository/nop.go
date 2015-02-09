// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package repository

func init() {
	Register("nop", nopManager{})
}

type nopManager struct{}

func (nopManager) CreateUser(username string) error {
	return nil
}

func (nopManager) RemoveUser(username string) error {
	return nil
}

func (nopManager) GrantAccess(repository, user string) error {
	return nil
}

func (nopManager) RevokeAccess(repository, user string) error {
	return nil
}

func (nopManager) AddKey(username string, key Key) error {
	return nil
}

func (nopManager) RemoveKey(username string, key Key) error {
	return nil
}

func (nopManager) ReadOnlyURL(repository string) (string, error) {
	return "", nil
}

func (nopManager) ReadWriteURL(repository string) (string, error) {
	return "", nil
}
