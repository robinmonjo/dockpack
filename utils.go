package main

import (
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"syscall"
)

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

func writePktLine(line string, w io.Writer) (int, error) {
	payload := []byte(line)
	head := []byte(fmt.Sprintf("%04x", len(payload)+4))
	return w.Write(append(head, payload...))
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
