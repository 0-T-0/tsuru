// Copyright 2014 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package docker

import (
	"bytes"
	"crypto"
	"encoding/json"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/tsuru/config"
	"github.com/tsuru/docker-cluster/cluster"
	"github.com/tsuru/docker-cluster/storage"
	"github.com/tsuru/tsuru/action"
	"github.com/tsuru/tsuru/fs"
	tsuruIo "github.com/tsuru/tsuru/io"
	"github.com/tsuru/tsuru/log"
	"github.com/tsuru/tsuru/provision"
	"github.com/tsuru/tsuru/safe"
	"io"
	"labix.org/v2/mgo/bson"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	dCluster *cluster.Cluster
	cmutex   sync.Mutex
	fsystem  fs.Fs
)

var (
	clusterNodes map[string]string
	segScheduler segregatedScheduler
)

func getDockerServers() []cluster.Node {
	servers, _ := config.GetList("docker:servers")
	nodes := []cluster.Node{}
	clusterNodes = make(map[string]string)
	for index, server := range servers {
		id := fmt.Sprintf("server%d", index)
		node := cluster.Node{
			ID:      id,
			Address: server,
		}
		nodes = append(nodes, node)
		clusterNodes[id] = server
	}
	return nodes
}

func dockerCluster() *cluster.Cluster {
	cmutex.Lock()
	defer cmutex.Unlock()
	var clusterStorage cluster.Storage
	if dCluster == nil {
		if redisServer, err := config.GetString("docker:scheduler:redis-server"); err == nil {
			prefix, _ := config.GetString("docker:scheduler:redis-prefix")
			if password, err := config.GetString("docker:scheduler:redis-password"); err == nil {
				clusterStorage = storage.AuthenticatedRedis(redisServer, password, prefix)
			} else {
				clusterStorage = storage.Redis(redisServer, prefix)
			}
		}
		var nodes []cluster.Node
		if segregate, _ := config.GetBool("docker:segregate"); segregate {
			dCluster, _ = cluster.New(&segScheduler, clusterStorage)
		} else {
			nodes = getDockerServers()
			dCluster, _ = cluster.New(nil, clusterStorage, nodes...)
		}
	}
	return dCluster
}

func filesystem() fs.Fs {
	if fsystem == nil {
		fsystem = fs.OsFs{}
	}
	return fsystem
}

// runCmd executes commands and log the given stdout and stderror.
func runCmd(cmd string, args ...string) (string, error) {
	out := bytes.Buffer{}
	err := executor().Execute(cmd, args, nil, &out, &out)
	log.Debugf("running the cmd: %s with the args: %s", cmd, args)
	if err != nil {
		return "", &cmdError{cmd: cmd, args: args, err: err, out: out.String()}
	}
	return out.String(), nil
}

func getPort() (string, error) {
	port, err := config.Get("docker:run-cmd:port")
	if err != nil {
		return "", err
	}
	return fmt.Sprint(port), nil
}

func urlToHost(urlStr string) string {
	url, _ := url.Parse(urlStr)
	host, _, _ := net.SplitHostPort(url.Host)
	return host
}

func getHostAddr(hostID string) string {
	nodes, err := dockerCluster().Nodes()
	if err != nil {
		log.Errorf("Error trying to list cluster nodes: %s", err.Error())
		return ""
	}
	for _, node := range nodes {
		if node.ID == hostID {
			return urlToHost(node.Address)
		}
	}
	return ""
}

func hostToNodeName(host string) (string, error) {
	nodes, err := dockerCluster().Nodes()
	if err != nil {
		return "", err
	}
	for _, node := range nodes {
		if urlToHost(node.Address) == host {
			return node.ID, nil
		}
	}
	return "", fmt.Errorf("Host `%s` not found", host)
}

type container struct {
	ID       string
	AppName  string
	Type     string
	IP       string
	HostAddr string
	HostPort string
	Status   string
	Version  string
	Image    string
	Name     string
}

func (c *container) getAddress() string {
	return fmt.Sprintf("http://%s:%s", c.HostAddr, c.HostPort)
}

func containerName() string {
	h := crypto.MD5.New()
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	return fmt.Sprintf("%x", h.Sum(nil))[:20]
}

// creates a new container in Docker.
func (c *container) create(app provision.App, imageId string, cmds []string, destinationHosts ...string) error {
	port, err := getPort()
	if err != nil {
		log.Errorf("error on getting port for container %s - %s", c.AppName, port)
		return err
	}
	user, _ := config.GetString("docker:ssh:user")
	exposedPorts := make(map[docker.Port]struct{}, 1)
	p := docker.Port(fmt.Sprintf("%s/tcp", port))
	exposedPorts[p] = struct{}{}
	gitUnitRepo, _ := config.GetString("git:unit-repo")
	sharedMount, _ := config.GetString("docker:sharedfs:mountpoint")
	sharedBasedir, _ := config.GetString("docker:sharedfs:hostdir")
	config := docker.Config{
		Image:        imageId,
		Cmd:          cmds,
		User:         user,
		ExposedPorts: exposedPorts,
		AttachStdin:  false,
		AttachStdout: false,
		AttachStderr: false,
		Memory:       int64(app.GetMemory() * 1024 * 1024),
		MemorySwap:   int64(app.GetSwap() * 1024 * 1024),
	}
	config.Env = append(config.Env, fmt.Sprintf("TSURU_APP_DIR=%s", gitUnitRepo))
	if sharedMount != "" && sharedBasedir != "" {
		config.Volumes = map[string]struct{}{
			sharedMount: {},
		}

		config.Env = append(config.Env, fmt.Sprintf("TSURU_SHAREDFS_MOUNTPOINT=%s", sharedMount))
	}
	opts := docker.CreateContainerOptions{Name: c.Name, Config: &config}
	var nodeList []string
	if len(destinationHosts) > 0 {
		nodeName, err := hostToNodeName(destinationHosts[0])
		if err != nil {
			return err
		}
		nodeList = []string{nodeName}
	}
	hostID, cont, err := dockerCluster().CreateContainerSchedulerOpts(opts, app.GetName(), nodeList...)
	if err != nil {
		log.Errorf("error on creating container in docker %s - %s", c.AppName, err)
		return err
	}
	c.ID = cont.ID
	c.HostAddr = getHostAddr(hostID)
	return nil
}

// networkInfo returns the IP and the host port for the container.
func (c *container) networkInfo() (string, string, error) {
	port, err := getPort()
	if err != nil {
		return "", "", err
	}
	dockerContainer, err := dockerCluster().InspectContainer(c.ID)
	if err != nil {
		return "", "", err
	}
	if dockerContainer.NetworkSettings != nil {
		ip := dockerContainer.NetworkSettings.IPAddress
		p := docker.Port(fmt.Sprintf("%s/tcp", port))
		for _, port := range dockerContainer.NetworkSettings.Ports[p] {
			if port.HostPort != "" && port.HostIp != "" {
				return ip, port.HostPort, nil
			}
		}
	}
	return "", "", fmt.Errorf("Container port %s is not mapped to any host port", port)
}

func (c *container) setStatus(status string) error {
	c.Status = status
	coll := collection()
	defer coll.Close()
	return coll.Update(bson.M{"id": c.ID}, c)
}

func (c *container) setImage(imageId string) error {
	c.Image = imageId
	coll := collection()
	defer coll.Close()
	return coll.Update(bson.M{"id": c.ID}, c)
}

func gitDeploy(app provision.App, version string, w io.Writer) (string, error) {
	commands, err := gitDeployCmds(app, version)
	if err != nil {
		return "", err
	}
	return deploy(app, commands, w)
}

func archiveDeploy(app provision.App, archiveURL string, w io.Writer) (string, error) {
	commands, err := archiveDeployCmds(app, archiveURL)
	if err != nil {
		return "", err
	}
	return deploy(app, commands, w)
}

func deploy(app provision.App, commands []string, w io.Writer) (string, error) {
	writer := tsuruIo.NewKeepAliveWriter(w, 30*time.Second, "please wait...")
	imageId := getImage(app)
	actions := []*action.Action{&insertEmptyContainerInDB, &createContainer, &startContainer, &updateContainerInDB, &followLogsAndCommit}
	pipeline := action.NewPipeline(actions...)
	err := pipeline.Execute(app, imageId, commands, []string{}, writer)
	if err != nil {
		log.Errorf("error on execute deploy pipeline for app %s - %s", app.GetName(), err)
		return "", err
	}
	return pipeline.Result().(string), nil
}

func start(app provision.App, imageId string, w io.Writer, destinationHosts ...string) (*container, error) {
	run_with_agent_commands, err := runWithAgentCmds(app)
	if err != nil {
		return nil, err
	}
	actions := []*action.Action{&insertEmptyContainerInDB, &createContainer, &startContainer, &updateContainerInDB, &setNetworkInfo, &addRoute}
	pipeline := action.NewPipeline(actions...)
	err = pipeline.Execute(app, imageId, run_with_agent_commands, destinationHosts)
	if err != nil {
		return nil, err
	}
	c := pipeline.Result().(container)
	err = c.setImage(imageId)
	if err != nil {
		return nil, err
	}
	err = c.setStatus("running")
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// remove removes a docker container.
func (c *container) remove() error {
	address := c.getAddress()
	log.Debugf("Removing container %s from docker", c.ID)
	err := dockerCluster().RemoveContainer(docker.RemoveContainerOptions{ID: c.ID})
	if err != nil {
		log.Errorf("Failed to remove container from docker: %s", err)
	}
	c.removeHost()
	log.Debugf("Removing container %s from database", c.ID)
	coll := collection()
	defer coll.Close()
	if err := coll.Remove(bson.M{"id": c.ID}); err != nil {
		log.Errorf("Failed to remove container from database: %s", err)
	}
	r, err := getRouter()
	if err != nil {
		log.Errorf("Failed to obtain router: %s", err)
	}
	if err := r.RemoveRoute(c.AppName, address); err != nil {
		log.Errorf("Failed to remove route: %s", err)
	}
	return nil
}

func (c *container) removeHost() error {
	url := fmt.Sprintf("http://%s:%d/container/%s", c.HostAddr, sshAgentPort(), c.IP)
	request, _ := http.NewRequest("DELETE", url, nil)
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *container) ssh(stdout, stderr io.Writer, cmd string, args ...string) error {
	ip, _, err := c.networkInfo()
	if err != nil {
		return err
	}
	stdout = &filter{w: stdout, content: []byte("unable to resolve host")}
	url := fmt.Sprintf("http://%s:%d/container/%s/cmd", c.HostAddr, sshAgentPort(), ip)
	input := cmdInput{Cmd: cmd, Args: args}
	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(input)
	if err != nil {
		return err
	}
	log.Debugf("Running SSH on %s:%d: %s %s", c.HostAddr, sshAgentPort(), cmd, args)
	resp, err := http.Post(url, "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(stdout, resp.Body)
	return err
}

// commit commits an image in docker based in the container
// and returns the image repository.
func (c *container) commit() (string, error) {
	log.Debugf("commiting container %s", c.ID)
	repository := assembleImageName(c.AppName)
	opts := docker.CommitContainerOptions{Container: c.ID, Repository: repository}
	image, err := dockerCluster().CommitContainer(opts)
	if err != nil {
		log.Errorf("Could not commit docker image: %s", err)
		return "", err
	}
	log.Debugf("image %s generated from container %s", image.ID, c.ID)
	pushImage(repository)
	return repository, nil
}

// stop stops the container.
func (c *container) stop() error {
	if c.Status == provision.StatusStopped.String() {
		return nil
	}
	err := dockerCluster().StopContainer(c.ID, 10)
	if err != nil {
		log.Errorf("error on stop container %s: %s", c.ID, err)
	}
	c.setStatus(provision.StatusStopped.String())
	return nil
}

func (c *container) start() error {
	port, err := getPort()
	if err != nil {
		return err
	}
	sharedBasedir, _ := config.GetString("docker:sharedfs:hostdir")
	sharedMount, _ := config.GetString("docker:sharedfs:mountpoint")
	sharedIsolation, _ := config.GetBool("docker:sharedfs:app-isolation")
	sharedSalt, _ := config.GetString("docker:sharedfs:salt")
	config := docker.HostConfig{}
	bindings := make(map[docker.Port][]docker.PortBinding)
	bindings[docker.Port(fmt.Sprintf("%s/tcp", port))] = []docker.PortBinding{
		{
			HostIp:   "",
			HostPort: "",
		},
	}
	config.PortBindings = bindings
	if sharedBasedir != "" && sharedMount != "" {
		if sharedIsolation {
			var appHostDir string
			if sharedSalt != "" {
				h := crypto.SHA1.New()
				io.WriteString(h, sharedSalt+c.AppName)
				appHostDir = fmt.Sprintf("%x", h.Sum(nil))
			} else {
				appHostDir = c.AppName
			}
			config.Binds = append(config.Binds, fmt.Sprintf("%s/%s:%s:rw", sharedBasedir, appHostDir, sharedMount))
		} else {
			config.Binds = append(config.Binds, fmt.Sprintf("%s:%s:rw", sharedBasedir, sharedMount))
		}
	}
	err = dockerCluster().StartContainer(c.ID, &config)
	if err != nil {
		return err
	}
	c.setStatus(provision.StatusStarted.String())
	return nil
}

// logs returns logs for the container.
func (c *container) logs(w io.Writer) error {
	opts := docker.AttachToContainerOptions{
		Container:    c.ID,
		Logs:         true,
		Stdout:       true,
		OutputStream: w,
		ErrorStream:  w,
		Stream:       true,
	}
	err := dockerCluster().AttachToContainer(opts)
	if err != nil {
		return err
	}
	opts = docker.AttachToContainerOptions{
		Container:    c.ID,
		Logs:         true,
		Stderr:       true,
		OutputStream: w,
		ErrorStream:  w,
	}
	return dockerCluster().AttachToContainer(opts)
}

// getImage returns the image name or id from an app.
// when the container image is empty is returned the platform image.
// when a deploy is multiple of 10 is returned the platform image.
func getImage(app provision.App) string {
	c, err := getOneContainerByAppName(app.GetName())
	if err != nil || c.Image == "" {
		return assembleImageName(app.GetPlatform())
	}
	if usePlatformImage(app) {
		err := removeImage(c.Image)
		if err != nil {
			log.Error(err.Error())
		}
		return assembleImageName(app.GetPlatform())
	}
	return c.Image
}

// removeImage removes an image from docker registry
func removeImage(imageId string) error {
	removeFromRegistry(imageId)
	return dockerCluster().RemoveImage(imageId)
}

func removeFromRegistry(imageId string) {
	parts := strings.SplitN(imageId, "/", 3)
	if len(parts) > 2 {
		registryServer := parts[0]
		url := fmt.Sprintf("http://%s/v1/repositories/%s/tags", registryServer,
			strings.Join(parts[1:], "/"))
		request, err := http.NewRequest("DELETE", url, nil)
		if err == nil {
			http.DefaultClient.Do(request)
		}
	}
}

type cmdError struct {
	cmd  string
	args []string
	err  error
	out  string
}

func (e *cmdError) Error() string {
	command := e.cmd + " " + strings.Join(e.args, " ")
	return fmt.Sprintf("Failed to run command %q (%s): %s.", command, e.err, e.out)
}

// pushImage sends the given image to the registry server defined in the
// configuration file.
func pushImage(name string) error {
	if _, err := config.GetString("docker:registry"); err == nil {
		var buf safe.Buffer
		pushOpts := docker.PushImageOptions{Name: name, OutputStream: &buf}
		err = dockerCluster().PushImage(pushOpts, docker.AuthConfiguration{})
		if err != nil {
			log.Errorf("[docker] Failed to push image %q (%s): %s", name, err, buf.String())
			return err
		}
	}
	return nil
}

func assembleImageName(appName string) string {
	parts := make([]string, 0, 3)
	registry, _ := config.GetString("docker:registry")
	if registry != "" {
		parts = append(parts, registry)
	}
	repoNamespace, _ := config.GetString("docker:repository-namespace")
	parts = append(parts, repoNamespace, appName)
	return strings.Join(parts, "/")
}

func usePlatformImage(app provision.App) bool {
	deploys := app.GetDeploys()
	if (deploys != 0 && deploys%10 == 0) || app.GetUpdatePlatform() {
		return true
	}
	return false
}
