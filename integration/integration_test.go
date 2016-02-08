package integration

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

const (
	sshPort = "2222"
)

// test a full git push without going through the push process
func TestGitPush(t *testing.T) {

	for _, env := range []string{"DOCKER_H", "DOCKER_HUB_USERNAME", "DOCKER_HUB_PASSWORD", "DOCKPACK_IMAGE"} {
		if os.Getenv(env) == "" {
			t.Fatalf("Missing %s env var", env)
		}
	}

	//mock a git repository
	dockerHost := os.Getenv("DOCKER_H")
	remote := fmt.Sprintf("ssh://%s:%s/test_app.git", dockerHost, sshPort)
	dir, err := mockGitRepo(remote)
	defer os.RemoveAll(dir)

	//start a http server to get back the hook
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(body))
	})

	go func() {
		if err := http.ListenAndServe(":9999", nil); err != nil {
			t.Fatal(err)
		}
	}()

	contID, err := startDockpack(sshPort, os.Getenv("DOCKER_HUB_USERNAME"), os.Getenv("DOCKER_HUB_PASSWORD"), "http://192.168.99.1", os.Getenv("DOCKPACK_IMAGE"))
	if err != nil {
		t.Fatal(err)
	}
	defer stopDockpack(contID)

	out, err := pushDockpack(dir)
	if err != nil {
		t.Fatalf("error: %v, output: %q", err, out)
	}
	fmt.Println(out)
}

//TODO: make sure only master branch is pushed

//TODO: test web hook
