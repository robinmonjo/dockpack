# dockpack

## Running

To run `dockpack`, you may pull the last image from the docker-hub (see Makefile for latest version) robinmonjo/dockpack:v, and start it with this command:

````bash
mkdir /home/ubuntu/sandbox
docker run -e REGISTRY_USERNAME="xxx" -e REGISTRY_PASSWORD="yyy" -e IMAGE_NAMESPACE="company_name" -e SSH_PORT=2222 -v /var/run/docker.sock:/var/run/docker.sock -v /home/ubuntu/sandbox:/sandbox -p 2222:2222 robinmonjo/dockpack:1.0
````

You can then add it as a remote on one of your project:

````bash
cd my/super/project
git remote add $remote ssh://$hostname:2222/packman.git
git push $remote master
````

All git repository and buildpacks cache will be persisted in the sandbox folder on the host.


## Options

**Registry authentication**

`dockpack` may interact with at most 2 registry servers:

- One to pull the build image (by default the [gliderlabs/herokuish](https://hub.docker.com/r/gliderlabs/herokuish/) image on the docker hub)
- One to push the built image (by default on the docker hub)

These info can be set using:

Pulling the image used to build:

- `PULL_REGISTRY_USERNAME` / `PULL_REGISTRY_PASSWORD` mandatory, refers to the credentials of the registry you wan to pull the image used to build
- `PULL_REGISTRY_SERVER` optional, refers to the registry server the image used to build is pull (default to the docker hub)

Pushing the built image:

- `PUSH_REGISTRY_USERNAME` / `PUSH_REGISTRY_PASSWORD` optional, refers to the credentials of the registry you want to push the built image (default to `PULL_REGISTRY_USERNAME` / `PULL_REGISTRY_PASSWORD`)
- `PUSH_REGISTRY_SERVER` optional, refers to the registry server the image built is pushed (default to `PULL_REGISTRY_SERVER`)

**Webhook**

If you pass the `WEB_HOOK` env to the container, a HTTP PUT request with the following body is made after each successful build:

````json
{
  "repo": "<repo_name>",
  "image_name": "<image_name>",
  "image_tag": "<image_tag>",
  "procfile": {
    "web": "bundle exec rails s",
    "worker" : "<some worker>"
  }
}
````

Note: `procfile` section may be empty if no procfile in the project

## Authentication

Authentication can be achieved through github. Use the `GITHUB_AUTH=true` to activate the authentication. You will need two more env:

- `GITHUB_AUTH_TOKEN` a [personal github access token](https://help.github.com/articles/creating-an-access-token-for-command-line-use)
- `GITHUB_OWNER` basically your github organization name

Note on authentication:

- ssh connection (git push) must be done with the github username of the person. You may need to set it in your remote (e.g: `ssh://<github_username>@<hostname>:<port>/<app_name>.git`)
- name of the repo on dockpack must match with the one on github

## Custom build image

`dockpack` relies on [herokuish](https://github.com/gliderlabs/herokuish) and therefore uses the [gliderlabs/herokuish](https://hub.docker.com/r/gliderlabs/herokuish/) docker image to pack your app. However you may need to customize this image ([example](https://github.com/applidget/dcdget-herokuish)). To pass you own image, you can set these environment variables:

- `BUILD_IMAGE` (default to `gliderlabs/herokuish`)
- `BUILD_IMAGE_TAG` (default to `latest`)

## Development

- You can dockerize the app using `make dockerize` and then just start the container and push onto it
- run integration tests using `make integration`
- run unit tests using `make tests`
