package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

const (
	endpoint      = "unix:///var/run/docker.sock"
	buildImage    = "gliderlabs/herokuish"
	buildImageTag = "latest"
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

	//check if herokuish latest exists
	authOpts := docker.AuthConfiguration{
		Username: os.Getenv("DOCKER_HUB_USERNAME"),
		Password: os.Getenv("DOCKER_HUB_PASSWORD"),
	}

	pullOpts := docker.PullImageOptions{
		Repository: buildImage,
		Tag:        buildImageTag,
	}

	b.writer.Write([]byte("-----> Pulling build image if required ...\r\n"))

	if err := b.client.PullImage(pullOpts, authOpts); err != nil {
		return err
	}

	//create a container for the build
	b.writer.Write([]byte("-----> Preparing build container\r\n"))
	createOpts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("%s_%s", b.appName, b.ref),
		Config: &docker.Config{
			Image: fmt.Sprintf("%s:%s", buildImage, buildImageTag),
			Cmd:   []string{"/build"},
		},
		HostConfig: &docker.HostConfig{},
	}
	container, err := b.client.CreateContainer(createOpts)
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

	//upload source code and cache (if any) inside the container
	b.writer.Write([]byte("-----> Uploading sources and cache into the container\r\n"))
	srcTarPath := filepath.Join("sandbox", fmt.Sprintf("%s_%s.tar", b.appName, b.ref))
	uploads := map[string]string{
		srcTarPath: "/tmp/build",
	}
	cachePath := filepath.Join("sandbox", fmt.Sprintf("%s_cache.tar", b.appName))
	if _, err := os.Stat(cachePath); err == nil {
		//cache tar exists
		uploads[cachePath] = "/tmp/"
	}

	for src, dest := range uploads {
		srcTar, err := os.Open(src)
		if err != nil {
			return err
		}
		defer srcTar.Close()

		uploadOpts := docker.UploadToContainerOptions{
			InputStream: srcTar,
			Path:        dest, //see herokuish doc for more informations
		}

		if err := b.client.UploadToContainer(container.ID, uploadOpts); err != nil {
			return err
		}
	}

	//tar can be removed, it's inside the build container
	if err := os.RemoveAll(srcTarPath); err != nil {
		return err
	}

	//start the container, this will start the build
	//TODO add env needed for the build (dcdget api)

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

	if statusCode != 0 {
		return fmt.Errorf("build container finished with status code: %d", statusCode)
	}

	//save the cache for next build
	b.writer.Write([]byte("-----> Saving cache for next build\r\n"))
	if err := os.RemoveAll(cachePath); err != nil {
		return err
	}
	cacheTar, err := os.Create(cachePath)
	if err != nil {
		return err
	}
	defer cacheTar.Close()
	dlOpts := docker.DownloadFromContainerOptions{
		Path:         "/tmp/cache",
		OutputStream: cacheTar,
	}
	if err := b.client.DownloadFromContainer(container.ID, dlOpts); err != nil {
		return err
	}

	//commit the container and upload the image, include a timestamp in the tag so it's ordered
	tag := fmt.Sprintf("%d_%s", time.Now().Unix(), b.ref)
	imgName := fmt.Sprintf("robinmonjo/%s", b.appName)
	ciOpts := docker.CommitContainerOptions{
		Container:  container.ID,
		Repository: imgName,
		Tag:        tag,
		Message:    "dockpack build",
		Author:     "dockpack",
		Run: &docker.Config{
			Cmd: []string{"/start", "web"},
		},
	}
	if _, err := b.client.CommitContainer(ciOpts); err != nil {
		return err
	}

	//prepare the image to be destroyed
	defer func() {
		img := fmt.Sprintf("%s:%s", imgName, tag)
		if err := b.client.RemoveImage(img); err != nil {
			log.Errorf("unable to remove image %s: %v", img, err)
		}
	}()

	pushOpts := docker.PushImageOptions{
		Name: imgName,
		Tag:  tag,
	}

	b.writer.Write([]byte(fmt.Sprintf("-----> Pushing image %s:%s to the registry (this may takes some times)\r\n", imgName, tag)))

	return b.client.PushImage(pushOpts, authOpts)
}
