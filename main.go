package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	log "github.com/Sirupsen/logrus"
)

var (
	version  string //set by the makefile
	sshPort  string
	httpPort string
)

const (
	buildErrorPrefix = "BUILD ERROR"
)

func init() {
	sshPort = os.Getenv("SSH_PORT")
	if sshPort == "" {
		sshPort = "9999"
	}

	var err error
	httpPort, err = freePort()
	if err != nil {
		panic(err)
	}
}

func main() {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)

		type body struct {
			Repo string `json:"repo"`
			Ref  string `json:"ref"`
		}
		var b body
		err := decoder.Decode(&b)
		if err != nil {
			log.Error(err)
		}
		log.Infof("Payload: %#v", b)
		handleApp(w, b.Repo, b.Ref)
	})

	go func() {
		if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
			log.Fatal(err)
		}
	}()

	//start ssh server
	s, err := newServer()
	if err != nil {
		log.Fatal(err)
	}

	if err := s.start(sshPort); err != nil {
		log.Fatal(err)
	}
}

type flushWriter struct {
	f http.Flusher
	w io.Writer
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return
}

func handleApp(w http.ResponseWriter, repo, ref string) {
	fw := &flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.f = f
	}

	//from here we should start the build and write output to w
	fw.Write([]byte(fmt.Sprintf("starting build for repo %s ref %s\n", repo, ref)))
	b, err := newBuilder(fw, repo, ref)
	if err != nil {
		log.Errorf("unable to instanciate builder: %v", err)
		fw.Write([]byte(fmt.Sprintf("unable to instanciate builder: %v\n", err)))
		return
	}

	br, err := b.build()
	if err != nil {
		log.Errorf("build failed: %v", err)
		fw.Write([]byte(fmt.Sprintf("%s - %v\n", buildErrorPrefix, err)))
		return
	}

	hook := os.Getenv("WEB_HOOK")
	if hook == "" {
		return
	}

	if err := put(hook, br, fw); err != nil {
		m := fmt.Sprintf("unable to notify hook %q: %v", hook, err)
		log.Errorf(m)
		fw.Write([]byte(m))
	}
}
