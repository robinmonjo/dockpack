package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const remoteName = "dockpack"

func git(args ...string) (string, error) {
	return run("git", args...)
}

func docker(args ...string) (string, error) {
	return run("docker", args...)
}

func run(bin string, args ...string) (string, error) {
	out, err := exec.Command(bin, args...).CombinedOutput()
	return strings.Trim(string(out), "\n"), err
}

//create a simple git repo, ready to be pushed, in a temporary folder and returns the path of the repo or an error
func mockGitRepo(remote string) (string, error) {
	dir, err := ioutil.TempDir("", "dockpack_")
	if err != nil {
		return "", err
	}

	if err := os.Chdir(dir); err != nil {
		return "", err
	}

	if _, err := git("init"); err != nil {
		return "", err
	}

	if err := mockRubyApp(dir); err != nil {
		return "", err
	}

	if _, err := git("add", "."); err != nil {
		return "", err
	}

	if _, err := git("commit", "-m", "some commit"); err != nil {
		return "", err
	}

	if _, err := git("remote", "add", remoteName, remote); err != nil {
		return "", err
	}

	return dir, nil
}

func mockRubyApp(dir string) error {
	if err := ioutil.WriteFile(filepath.Join(dir, "README.md"), []byte("# my awesome project"), 0777); err != nil {
		return err
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "Gemfile.lock"), []byte(""), 0777); err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(dir, "Gemfile"), []byte("source 'https://rubygems.org'\nruby '2.2.3'"), 0777)
}

func pushDockpack(repo string) (string, error) {
	if err := os.Chdir(repo); err != nil {
		return "", err
	}
	return git("push", remoteName, "master")
}

func startDockpack(port, dhUser, dhPasswd, namespace, webHook, image string) (string, error) {
	return docker("run",
		"-e", fmt.Sprintf("REGISTRY_USERNAME=%s", dhUser),
		"-e", fmt.Sprintf("REGISTRY_PASSWORD=%s", dhPasswd),
		"-e", fmt.Sprintf("IMAGE_NAMESPACE=%s", namespace),
		"-e", fmt.Sprintf("SSH_PORT=%s", port),
		"-e", "DOCKPACK_ENV=testing",
		"-e", fmt.Sprintf("WEB_HOOK=%s", webHook),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-p", fmt.Sprintf("%s:%s", port, port),
		"-d",
		image,
	)
}

func stopDockpack(contID string) error {
	_, err := docker("stop", contID)
	return err
}
