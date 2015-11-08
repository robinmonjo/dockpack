package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	pushCmd = "git-receive-pack"
	pullCmd = "git-upload-pack"
)

type server struct {
	config     *ssh.ServerConfig
	workingDir string
}

func newServer() (*server, error) {
	config := &ssh.ServerConfig{}
	config.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		return &ssh.Permissions{}, nil
	}

	keyPath := "./id_rsa"
	pkBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	pk, err := ssh.ParsePrivateKey(pkBytes)
	if err != nil {
		return nil, err
	}
	config.AddHostKey(pk)
	workingDir, err := filepath.Abs("./sandbox")
	if err != nil {
		return nil, err
	}

	return &server{
		config:     config,
		workingDir: workingDir,
	}, nil
}

func (s *server) start(port string) error {
	socket, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Server listening on :%s", port))
	for {
		conn, err := socket.Accept()
		if err != nil {
			log.Error(err)
			continue
		}

		// From a standard TCP connection to an encrypted SSH connection
		sshConn, newChans, _, err := ssh.NewServerConn(conn, s.config)
		if err != nil {
			log.Error(err)
			continue
		}
		defer sshConn.Close()

		log.Info(fmt.Sprintf("connection from %s", sshConn.RemoteAddr()))
		go func() {
			for chanReq := range newChans {
				go s.handleChanReq(chanReq)
			}
		}()
	}
}

func (s *server) handleChanReq(chanReq ssh.NewChannel) {
	if chanReq.ChannelType() != "session" {
		chanReq.Reject(ssh.Prohibited, "channel type is not a session")
		return
	}

	ch, reqs, err := chanReq.Accept()
	if err != nil {
		log.Error("fail to accept channel request", err)
		return
	}

	for {
		req := <-reqs

		switch req.Type {
		case "env":
		case "exec":
			s.handleExec(ch, req)
			return
		default:
			ch.Write([]byte(fmt.Sprintf("request type %q not allowed\r\n", req.Type)))
			ch.Close()
			return
		}
	}
}

func (s *server) handleExec(ch ssh.Channel, req *ssh.Request) {
	defer ch.Close()

	args := strings.SplitN(string(req.Payload[4:]), " ", 2) //remove the 4 bytes of git protocol indicating line length
	command := args[0]
	repoName := strings.TrimSuffix(strings.TrimPrefix(args[1], "'/"), ".git'")

	//check if allowed command
	allowed := []string{pullCmd, pushCmd}
	ok := false
	for _, c := range allowed {
		if command == c {
			ok = true
			break
		}
	}

	if !ok {
		ch.Write([]byte(fmt.Sprintf("%s not allowed on this server\r\n", command)))
		return
	}

	repoPath, err := s.createRepoIfNeeded(repoName)
	if err != nil {
		log.Error(err)
		ch.Write([]byte(err.Error() + "\r\n"))
		return
	}

	//always inject pre-receive hook as http port may changes
	if err := s.injectPreReceiveHook(repoName); err != nil {
		log.Error(err)
		ch.Write([]byte(err.Error() + "\r\n"))
		return
	}

	cmd := exec.Command(command, repoPath)
	wg, err := attachCmd(cmd, ch)
	if err != nil {
		ch.Write([]byte(err.Error() + "\r\n"))
		return
	}

	if err := cmd.Start(); err != nil {
		ch.Write([]byte(err.Error() + "\r\n"))
		return
	}
	wg.Wait()
	syscallErr := cmd.Wait()

	ch.SendRequest("exit-status", false, ssh.Marshal(exitStatus(syscallErr)))
}

func (s *server) createRepoIfNeeded(name string) (string, error) {
	path := filepath.Join(s.workingDir, name)

	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	cmd := exec.Command("git", "--git-dir="+path, "init", "--bare")
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}

	return path, nil
}

func (s *server) injectPreReceiveHook(appName string) error {
	path := filepath.Join(s.workingDir, appName, "hooks", "pre-receive")
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Chmod(0777); err != nil {
		return err
	}

	const script = `#!/bin/sh
set -e
while read old_ref new_ref ref_name
do
  if [[ $ref_name = "refs/heads/master" ]]; then
    git archive -o {{.ArchiveFolder}}/{{.AppName}}_$new_ref.tar $new_ref
    curl -N -s -m 3600 -X PUT -H 'Content-Type: application/json' -d "{\"app_name\": \"{{.AppName}}\", \"ref\": \"$new_ref\"}" {{.Endpoint}}
  fi
done

exit 0
  `
	type hookData struct {
		AppName       string
		Endpoint      string
		ArchiveFolder string
	}

	data := hookData{
		AppName:       appName,
		Endpoint:      fmt.Sprintf("localhost:%s", httpPort),
		ArchiveFolder: s.workingDir,
	}

	return template.Must(template.New("hook").Parse(script)).Execute(f, data)
}

func attachCmd(cmd *exec.Cmd, ch ssh.Channel) (*sync.WaitGroup, error) {
	var wg sync.WaitGroup
	wg.Add(3)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	go func() {
		defer wg.Done()
		io.Copy(stdin, ch)
	}()

	go func() {
		defer wg.Done()
		io.Copy(ch.Stderr(), stderr)
	}()

	go func() {
		defer wg.Done()
		io.Copy(ch, stdout)
	}()

	return &wg, nil
}

type exitStatusResp struct {
	Status uint32
}

func exitStatus(err error) exitStatusResp {
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return exitStatusResp{uint32(status.ExitStatus())}
			}
		}
		return exitStatusResp{1} //not a syscall err, but err anyway
	}
	return exitStatusResp{0}
}
