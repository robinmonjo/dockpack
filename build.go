package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
)

const (
	endpoint = "unix:///var/run/docker.sock"
)

var (
	buildImage    = "gliderlabs/herokuish"
	buildImageTag = "latest"

	pullAuthOpts docker.AuthConfiguration
	pushAuthOpts docker.AuthConfiguration
)

func init() {
	if image := os.Getenv("BUILD_IMAGE"); image != "" {
		buildImage = image
	}

	if tag := os.Getenv("BUILD_IMAGE_TAG"); tag != "" {
		buildImageTag = tag
	}

	//pull auth (to get the build image)
	pullAuthOpts = docker.AuthConfiguration{
		Username:      os.Getenv("PULL_REGISTRY_USERNAME"),
		Password:      os.Getenv("PULL_REGISTRY_PASSWORD"),
		ServerAddress: os.Getenv("PULL_REGISTRY_SERVER"), //if empty will use default docker hub
	}

	//push auth (to push the built image)
	pushAuthOpts = docker.AuthConfiguration{}
	if username := os.Getenv("PUSH_REGISTRY_USERNAME"); username != "" {
		pushAuthOpts.Username = username
	} else {
		pushAuthOpts.Username = pullAuthOpts.Username
	}

	if pwd := os.Getenv("PUSH_REGISTRY_PASSWORD"); pwd != "" {
		pushAuthOpts.Password = pwd
	} else {
		pushAuthOpts.Password = pullAuthOpts.Password
	}

	if server := os.Getenv("PUSH_REGISTRY_SERVER"); server != "" {
		pushAuthOpts.ServerAddress = server
	} else {
		pushAuthOpts.ServerAddress = pullAuthOpts.ServerAddress
	}

}

type builder struct {
	client *docker.Client
	repo   string
	ref    string
	writer io.Writer
}

type buildResult struct {
	Repo      string            `json:"repo"`
	ImageName string            `json:"image_name"`
	ImageTag  string            `json:"image_tag"`
	Procfile  map[string]string `json:"procfile,omitempty"`
}

func newBuilder(w io.Writer, repo, ref string) (*builder, error) {
	client, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, err
	}
	return &builder{
		client: client,
		repo:   repo,
		ref:    ref,
		writer: w,
	}, nil
}

func (b *builder) build() (*buildResult, error) {

	//check if herokuish latest exists
	pullOpts := docker.PullImageOptions{
		Repository: buildImage,
		Tag:        buildImageTag,
	}

	b.logLine(fmt.Sprintf("-----> Pulling %s:%s image if required ...", buildImage, buildImageTag))

	if err := b.client.PullImage(pullOpts, pullAuthOpts); err != nil {
		return nil, err
	}

	//create a container for the build
	b.logLine("-----> Preparing build container")
	createOpts := docker.CreateContainerOptions{
		Name: fmt.Sprintf("%s_%s", b.repo, b.ref),
		Config: &docker.Config{
			Image: fmt.Sprintf("%s:%s", buildImage, buildImageTag),
			Cmd:   []string{"/build"},
		},
		HostConfig: &docker.HostConfig{},
	}
	container, err := b.client.CreateContainer(createOpts)
	if err != nil {
		return nil, err
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
	b.logLine("-----> Uploading sources and cache into the container")
	srcTarPath := filepath.Join("sandbox", fmt.Sprintf("%s_%s.tar", b.repo, b.ref))
	uploads := map[string]string{
		srcTarPath: "/tmp/build",
	}
	cachePath := filepath.Join("sandbox", fmt.Sprintf("%s_cache.tar", b.repo))
	if _, err := os.Stat(cachePath); err == nil {
		//cache tar exists
		uploads[cachePath] = "/tmp/"
	}

	for src, dest := range uploads {
		srcTar, err := os.Open(src)
		if err != nil {
			return nil, err
		}
		defer srcTar.Close()

		uploadOpts := docker.UploadToContainerOptions{
			InputStream: srcTar,
			Path:        dest, //see herokuish doc for more informations
		}

		if err := b.client.UploadToContainer(container.ID, uploadOpts); err != nil {
			return nil, err
		}
	}

	//tar can be removed, it's inside the build container
	if err := os.RemoveAll(srcTarPath); err != nil {
		return nil, err
	}

	//start the container, this will start the build
	if err := b.client.StartContainer(container.ID, &docker.HostConfig{}); err != nil {
		return nil, err
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
		return nil, err
	}

	//wait until the container stops and check if everything went fine
	statusCode, err := b.client.WaitContainer(container.ID)
	if err != nil {
		return nil, err
	}

	if statusCode != 0 {
		return nil, fmt.Errorf("build container finished with status code: %d", statusCode)
	}

	//save the cache for next build
	b.logLine("-----> Saving cache for next build")
	if err := os.RemoveAll(cachePath); err != nil {
		return nil, err
	}
	cacheTar, err := os.Create(cachePath)
	if err != nil {
		return nil, err
	}
	defer cacheTar.Close()
	dlOpts := docker.DownloadFromContainerOptions{
		Path:         "/tmp/cache",
		OutputStream: cacheTar,
	}
	if err := b.client.DownloadFromContainer(container.ID, dlOpts); err != nil {
		return nil, err
	}

	//commit the container and upload the image, include a timestamp in the tag so it's ordered
	tag := fmt.Sprintf("%d_%s", time.Now().Unix(), b.ref)
	imgName := fmt.Sprintf("%s/%s", os.Getenv("IMAGE_NAMESPACE"), b.repo)
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
		return nil, err
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

	b.logLine(fmt.Sprintf("-----> Pushing image %s:%s to the registry (this may takes some times)", imgName, tag))

	if os.Getenv("DOCKPACK_ENV") == "testing" {
		b.logLine(fmt.Sprintf("-----> Test, skipping push\r\n", imgName, tag))
	} else {
		if err := b.client.PushImage(pushOpts, pushAuthOpts); err != nil {
			return nil, err
		}
	}

	procfile, err := b.parseProcfile()
	if err != nil {
		b.logLine(fmt.Sprintf("No Procfile found or Procfile mal formated: %v", err))
	}

	fmt.Println(procfile)

	return &buildResult{Repo: b.repo, ImageName: imgName, ImageTag: tag, Procfile: procfile}, nil
}

func (b *builder) parseProcfile() (map[string]string, error) {
	procfile := filepath.Join("sandbox", fmt.Sprintf("%s_clone", b.repo), "Procfile")

	file, err := os.Open(procfile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	res := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 0 {
			comps := strings.SplitN(line, ":", 2)
			res[comps[0]] = comps[1]
		}
	}

	return res, scanner.Err()
}

func (b *builder) logLine(line string) {
	b.writer.Write([]byte(line + "\r\n"))
}
