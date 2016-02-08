package auth

import (
	"fmt"
	"testing"
)

func TestNewGithubAuth(t *testing.T) {
	a, err := NewGithubAuth()
	if err != nil {
		t.Fatal(err)
	}

	colls, err := a.fetchWriteCollaborators("mobicheckin-server")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(colls)

	a.Authenticate("lol", "mobicheckin-server")

}
