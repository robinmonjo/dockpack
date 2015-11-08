package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

const (
	endpoint   = "unix:///var/run/docker.sock"
	buildImage = "gliderlabs/herokuish:latest"
)

type builder struct {
	client  *docker.Client
	appName string
	ref     string
	writer  io.Writer
}

func newBuilder(w io.Writer, appName, ref string) (*builder, error) {
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return &builder{
		client:  client,
		appName: appName,
		ref:     ref,
		writer:  w,
	}, nil
}

func (b *builder) build() error {
	//create a container for the build
	opts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("%s_%s", b.appName, b.ref),
		Config: &docker.Config{
			Image: buildImage,
			Cmd:   []string{"/build"},
		},
		HostConfig: &docker.HostConfig{},
	}
	container, err := b.client.CreateContainer(opts)
	if err != nil {
		return err
	}

	//destroy it when finish
	defer func() {
		rmOpts := docker.RemoveContainerOptions{
			ID:    container.ID,
			Force: true,
		}
		if err := b.client.RemoveContainer(rmOpts); err != nil {
			log.Errorf("unable to remove container: %v", err)
		}
	}()

	//upload source code inside the container
	f, err := os.Open(filepath.Join("sandbox", fmt.Sprintf("%s_%s.tar", b.appName, b.ref)))
	if err != nil {
		return err
	}
	defer f.Close()

	uploadOpts := docker.UploadToContainerOptions{
		InputStream: f,
		Path:        "/tmp/build",
	}

	if err := b.client.UploadToContainer(container.ID, uploadOpts); err != nil {
		return err
	}

	//start the container, this will start the build
	if err := b.client.StartContainer(container.ID, &docker.HostConfig{}); err != nil {
		return err
	}

	//get back container logs and write them directly back to the client
	logOpts := docker.LogsOptions{
		Container:    container.ID,
		OutputStream: b.writer,
		ErrorStream:  b.writer,
		Follow:       true,
		Stdout:       true,
		Stderr:       true,
	}

	if err := b.client.Logs(logOpts); err != nil {
		return err
	}

	//wait until the container stops and check if everything went fine
	statusCode, err := b.client.WaitContainer(container.ID)
	if err != nil {
		return err
	}

	if statusCode == 0 {
		//good, we can ulpoad the image to the repository :)
	} else {
		//buildpack failed, we should reject the push (curl shouldn't exit with a 0 status code ...)
	}
	log.Infof("status code: %d", statusCode)

	return nil
}
