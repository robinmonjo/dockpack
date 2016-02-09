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
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/robinmonjo/dockpack/auth"
	"golang.org/x/crypto/ssh"
)

const (
	pushCmd      = "git-receive-pack"
	pullCmd      = "git-upload-pack"
	lockFile     = ".dockpack_lock"
	publicKeyKey = "pub_key"
)

type server struct {
	config     *ssh.ServerConfig
	workingDir string
}

func newServer() (*server, error) {
	config := &ssh.ServerConfig{}
	config.PublicKeyCallback = func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		//storing public key and user for authorization processing
		pk := ssh.MarshalAuthorizedKey(key)
		pk = pk[:len(pk)-1] //remove the trailling \n

		authInfo := map[string]string{
			"user":       c.User(),
			"public_key": string(pk),
		}

		return &ssh.Permissions{CriticalOptions: authInfo}, nil
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
	log.Infof("Server listening on :%s :)", port)
	for {
		conn, err := socket.Accept()
		if err != nil {
			log.Errorf("unable to accept connection %v", err)
			continue
		}

		// From a standard TCP connection to an encrypted SSH connection
		sshConn, newChans, _, err := ssh.NewServerConn(conn, s.config)
		if err != nil {
			log.Errorf("ssh handshake failed, %v", err)
			continue
		}
		defer sshConn.Close()

		log.Infof("connection from %s", sshConn.RemoteAddr())
		go func() {
			for chanReq := range newChans {
				go s.handleChanReq(chanReq, sshConn.Permissions.CriticalOptions)
			}
		}()
	}
}

func (s *server) handleChanReq(chanReq ssh.NewChannel, authInfo map[string]string) {
	if chanReq.ChannelType() != "session" {
		chanReq.Reject(ssh.Prohibited, "channel type is not a session")
		return
	}

	ch, reqs, err := chanReq.Accept()
	if err != nil {
		log.Errorf("fail to accept channel request %v", err)
		return
	}

	for {
		req := <-reqs

		switch req.Type {
		case "env":
		case "exec":
			s.handleExec(ch, req, authInfo)
			return
		default:
			ch.Write([]byte(fmt.Sprintf("request type %q not allowed\r\n", req.Type)))
			ch.Close()
			return
		}
	}
}

func (s *server) handleExec(ch ssh.Channel, req *ssh.Request, authInfo map[string]string) {
	defer ch.Close()
	args := strings.SplitN(string(req.Payload[4:]), " ", 2) //remove the 4 bytes of git protocol indicating line length
	command := args[0]
	repoName := strings.TrimSuffix(strings.TrimPrefix(args[1], "'/"), ".git'")

	//auth the user
	if os.Getenv("GITHUB_AUTH") == "true" {
		gauth, err := auth.NewGithubAuth()
		if err != nil {
			writePktLine(fmt.Sprintf("github auth error, contact an administrator: %s", err), ch)
			return
		}

		if err := gauth.Authenticate(authInfo["user"], authInfo["public_key"], repoName); err != nil {
			writePktLine(fmt.Sprintf("github auth failed: %s", err), ch)
			return
		}
	}

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
		log.Infof("command %s not allowed on this server", command)
		writePktLine(fmt.Sprintf("%s not allowed on this server", command), ch)
		return
	}

	log.Infof("receiving %s command for repo %s", command, repoName)

	repoPath, err := s.prepareRepo(repoName)
	if err != nil {
		log.Errorf("unable to create repo: %v", err)
		writePktLine(err.Error(), ch)
		return
	}

	defer func() {
		if err := s.unlockRepo(repoName); err != nil {
			log.Errorf("unable to unlock repo: %v", err)
			writePktLine(err.Error(), ch)
		}
	}()

	cmd := exec.Command(command, repoPath)
	wg, err := attachCmd(cmd, ch)
	if err != nil {
		log.Errorf("unable to attach command stdio: %v", err)
		writePktLine(err.Error(), ch)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Errorf("unable to start command: %v", err)
		writePktLine(err.Error(), ch)
		return
	}
	wg.Wait()
	syscallErr := cmd.Wait()

	ch.SendRequest("exit-status", false, ssh.Marshal(exitStatus(syscallErr)))
}

func (s *server) prepareRepo(repoName string) (string, error) {
	var lock = &sync.Mutex{}
	lock.Lock()
	defer lock.Unlock()

	repoPath, err := s.createRepoIfNeeded(repoName)
	if err != nil {
		return "", err
	}

	var err2 error
	if err := s.lockRepo(repoName); err != nil {
		return "", err
	}
	defer func() {
		if err2 != nil {
			if err := s.unlockRepo(repoName); err != nil {
				log.Errorf("unable to unlock repo: %v", err)
			}
		}
	}()

	//always inject pre-receive hook as http port may changes
	err2 = s.injectPreReceiveHook(repoName)
	return repoPath, err2
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
    curl -N -s -m 3600 -X PUT -H 'Content-Type: application/json' -d "{\"app_name\": \"{{.AppName}}\", \"ref\": \"$new_ref\"}" {{.Endpoint}} | tee {{.BuildLogs}}
		if grep -q "{{.BuildErrorPrefix}}" {{.BuildLogs}} ; then
			exit 1
		fi
  fi
done

exit 0
  `
	type hookData struct {
		AppName          string
		Endpoint         string
		ArchiveFolder    string
		BuildLogs        string
		BuildErrorPrefix string
	}

	data := hookData{
		AppName:          appName,
		Endpoint:         fmt.Sprintf("localhost:%s", httpPort),
		ArchiveFolder:    s.workingDir,
		BuildLogs:        filepath.Join(s.workingDir, fmt.Sprintf("%s.log", appName)),
		BuildErrorPrefix: buildErrorPrefix,
	}

	return template.Must(template.New("hook").Parse(script)).Execute(f, data)
}

func (s *server) lockFilePath(appName string) string {
	return filepath.Join(s.workingDir, appName, lockFile)
}

func (s *server) lockRepo(appName string) error {
	lockFilePath := s.lockFilePath(appName)
	if _, err := os.Stat(lockFilePath); err == nil {
		return fmt.Errorf("repo is locked, try again later")
	}
	_, err := os.Create(lockFilePath)
	return err
}

func (s *server) unlockRepo(appName string) error {
	return os.RemoveAll(s.lockFilePath(appName))
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
