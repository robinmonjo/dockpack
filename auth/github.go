package auth

import (
	"fmt"
	"os"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	githubAuthTokenEnv = "GITHUB_AUTH_TOKEN"
	githubOwnerEnv     = "GITHUB_OWNER"
)

type GithubAuth struct {
	Client  *github.Client
	Owner   string
	AppName string
	PubKey  string
}

func NewGithubAuth() (*GithubAuth, error) {
	for _, env := range []string{githubAuthTokenEnv, githubOwnerEnv} {
		if os.Getenv(env) == "" {
			return nil, fmt.Errorf("%s env must be set", env)
		}
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv(githubAuthTokenEnv)},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)

	return &GithubAuth{
		Client: github.NewClient(tc),
		Owner:  os.Getenv(githubOwnerEnv),
	}, nil
}

func (auth *GithubAuth) fetchWriteCollaborators(repo string) ([]github.User, error) {
	colls, _, err := auth.Client.Repositories.ListCollaborators(auth.Owner, repo, nil)
	if err != nil {
		return nil, err
	}
	writeColls := []github.User{}
	for _, coll := range colls {
		if (*coll.Permissions)["push"] {
			writeColls = append(writeColls, coll)
		}
	}
	return writeColls, nil
}

func (auth *GithubAuth) Authenticate(pubKey, repo string) error {
	colls, err := auth.fetchWriteCollaborators(repo)
	if err != nil {
		return err
	}

	for _, coll := range colls {
		keys, _, err := auth.Client.Users.ListKeys(*coll.Login, nil)
		if err != nil {
			return err
		}

		for _, key := range keys {
			if *key.Key == pubKey {
				//auth successfull
				return nil
			}
		}
	}

	return fmt.Errorf("not authorized to push on %s", repo)
}
