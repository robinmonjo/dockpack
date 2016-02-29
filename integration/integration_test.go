package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
)

const (
	sshPort = "2222"
)

// test a full git push without going through the push image process
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
	var wg sync.WaitGroup
	wg.Add(1)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)

		var payload map[string]string

		if err := decoder.Decode(&payload); err != nil {
			t.Fatal(err)
		}

		for _, key := range []string{"repo", "image_name", "image_tag"} {
			if payload[key] == "" {
				t.Fatalf("expected key %q to exist in post build hook payload. Got %#v", key, payload)
			}
		}

		wg.Done()
	})

	go func() {
		if err := http.ListenAndServe(":9999", nil); err != nil {
			t.Fatal(err)
		}
	}()

	contID, err := startDockpack(sshPort, os.Getenv("DOCKER_HUB_USERNAME"), os.Getenv("DOCKER_HUB_PASSWORD"), "http://192.168.99.1:9999", os.Getenv("DOCKPACK_IMAGE"))
	if err != nil {
		t.Fatal(err)
	}
	defer stopDockpack(contID)

	out, err := pushDockpack(dir)
	if err != nil {
		t.Fatalf("error: %v, output: %q", err, out)
	}

	fmt.Println(out)

	//wait for the hook to be sent
	wg.Wait()
}

//TODO: make sure only master branch is pushed
