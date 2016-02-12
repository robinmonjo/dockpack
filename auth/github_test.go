package auth

import (
	"testing"
)

//modify constants to tests
const (
	allowedPubKey = "<SET ME>"

	notAllowedPubKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCZHxXUOXSGvbTGuY+aJLc9UPN2KsZbu9KAjBot78PWPJdqNSPzHMBnp86kWsa9SpE+mcBMFV6gOnNoFGW2OguKmfu60JIfTU/I51J8ns0nitmbuVg1MRBs9iN9AU/stZvLryEjwkzjQlrgu13jj8XJzn4DjO7gggggko/lIpBX8c3eYuzyOEl2oN4cGW6IoLTC3k5To9/6ddS+zD3k4vMA+1cRuMNDRowEAR6leZD+FxFCnqW60zyYvJCvp2hfq9N1keJrFfPoW3pI0/adqtB8kMGWGLLR2V9PAMOXKneCY19CpJM/OTi2jjy66Oid2E+TYJ3HLEVpecOsZf8yz+B3"

	allowedUser = "<SET ME>"

	notAllowedUser = "foobar"

	repo = "<SET ME>"
)

func TestNewGithubAuth(t *testing.T) {
	_, err := NewGithubAuth()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthenticate(t *testing.T) {
	a, err := NewGithubAuth()
	if err != nil {
		t.Fatal(err)
	}

	//should works
	if err := a.Authenticate(allowedUser, allowedPubKey, repo); err != nil {
		t.Fatal(err)
	}

	//should not work
	if err := a.Authenticate(allowedUser, notAllowedPubKey, repo); err == nil {
		t.Fatalf("authentication didn't failed on a not allowed public key")
	}

	if err := a.Authenticate(notAllowedUser, allowedPubKey, repo); err == nil {
		t.Fatalf("authentication didn't failed on an unknown user")
	}
}
