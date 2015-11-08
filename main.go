package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
)

var (
	version  string //set by the makefile
	sshPort  string
	httpPort string
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
			AppName string `json:"app_name"`
			Ref     string `json:"ref"`
		}
		var b body
		err := decoder.Decode(&b)
		if err != nil {
			log.Error(err)
		}
		log.Infof("Payload: %#v", b)
		handleApp(w, b.AppName, b.Ref)
	})

	go func() {
		err := http.ListenAndServe(":"+httpPort, nil)
		if err != nil {
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

func handleApp(w http.ResponseWriter, name, ref string) {
	fw := &flushWriter{w: w}
	if f, ok := w.(http.Flusher); ok {
		fw.f = f
	}

	//from here we should start the build and write output to w
	fw.Write([]byte(fmt.Sprintf("starting build for app %s ref %s\n", name, ref)))
	b, err := newBuilder(fw, name, ref)
	if err != nil {
		fw.Write([]byte(fmt.Sprintf("unable to instanciate builder: %v", err)))
	}
	if err := b.build(); err != nil {
		log.Error(err)
	}
}

//return a free port
func freePort() (string, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strings.TrimPrefix(l.Addr().String(), "[::]:"), nil
}