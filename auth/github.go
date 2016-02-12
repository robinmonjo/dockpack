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
	Client *github.Client
	Owner  string
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

func (auth *GithubAuth) checkUserIsWriteCollaborator(user, repo string) error {
	colls, _, err := auth.Client.Repositories.ListCollaborators(auth.Owner, repo, nil)
	if err != nil {
		return err
	}
	for _, coll := range colls {
		if (*coll.Login) == user && (*coll.Permissions)["push"] {
			return nil
		}
	}
	return fmt.Errorf("not authorized to push on %s", repo)
}

func (auth *GithubAuth) checkPublicKey(user, pubKey string) error {
	keys, _, err := auth.Client.Users.ListKeys(user, nil)
	if err != nil {
		return err
	}

	//check if key match
	keyMatch := false
	for _, key := range keys {
		keyMatch = (*key.Key == pubKey)
		if keyMatch {
			return nil
		}
	}
	return fmt.Errorf("permission denied (public key)")
}

func (auth *GithubAuth) Authenticate(user, pubKey, repo string) error {
	if err := auth.checkPublicKey(user, pubKey); err != nil {
		return err
	}

	return auth.checkUserIsWriteCollaborator(user, repo)
}
