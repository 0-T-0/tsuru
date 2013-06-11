// Copyright 2013 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"fmt"
	"github.com/globocom/config"
	"github.com/globocom/tsuru/provision"
	"io/ioutil"
	"os"
	"os/user"
	"path"
)

// deployCmds returns the commands that is used when provisioner
// deploy an unit.
func deployCmds(app provision.App) ([]string, error) {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return nil, err
	}
	deployCmd, err := config.GetString("docker:deploy-cmd")
	if err != nil {
		return nil, err
	}
	imageName := getImage(app)
	cmds := []string{docker, "run", imageName, deployCmd}
	return cmds, nil
}

// runCmds returns the commands that should be passed when the
// provisioner will run an unit.
func runCmds(app provision.App, imageId string) ([]string, error) {
	docker, err := config.GetString("docker:binary")
	if err != nil {
		return nil, err
	}
	runCmd, err := config.GetString("docker:run-cmd:bin")
	if err != nil {
		return nil, err
	}
	port, err := config.GetString("docker:run-cmd:port")
	if err != nil {
		return nil, err
	}
	cmds := []string{docker, "run", "-d", "-t", "-p", port, imageId, "/bin/bash", "-c", runCmd}
	return cmds, nil
}

// sshCmds returns the commands needed to start a ssh daemon.
func sshCmds() ([]string, error) {
	addKeyCommand, err := config.GetString("docker:ssh:add-key-cmd")
	if err != nil {
		return nil, err
	}
	keyFile, err := config.GetString("docker:ssh:public-key")
	if err != nil {
		if u, err := user.Current(); err == nil {
			keyFile = path.Join(u.HomeDir, ".ssh", "id_rsa.pub")
		} else {
			keyFile = os.ExpandEnv("${HOME}/.ssh/id_rsa.pub")
		}
	}
	f, err := filesystem().Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	keyContent, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	sshdPath, err := config.GetString("docker:ssh:sshd-path")
	if err != nil {
		sshdPath = "/usr/sbin/sshd"
	}
	return []string{
		fmt.Sprintf("%s %s", addKeyCommand, bytes.TrimSpace(keyContent)),
		sshdPath + " -D",
	}, nil
}
