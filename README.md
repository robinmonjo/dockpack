# dockpack

## Running

To run dockpack, you may pull the last image from the docker-hub (see Makefile for latest version) robinmonjo/dockpack:v, and start it with this command:

````bash
mkdir /home/ubuntu/sandbox
docker run -e DOCKER_HUB_USERNAME="xxx" -e DOCKER_HUB_PASSWORD="yyy" -e SSH_PORT=2222 -v /var/run/docker.sock:/var/run/docker.sock -v /home/ubuntu/sandbox:/sandbox -p $PORT:$PORT robinmonjo/dockpack:1.0
````

This will start a git server listening on 2222. You can then add it as a remote on one of your project:

````bash
cd my/super/project
git remote add $remote ssh://$hostname:2222/packman.git
git push $remote master
````

All git repository and buildpacks cache will be persisted in the sandbox folder on the host.

If you pass the `WEB_HOOK` env to the container, a HTTP PUT request with the following body is made after each successful build:

````json
{
  "imageName": "<image_name>",
  "imageTag": "<tag>"
}
````

## Development

- You can dockerize the app using `make dockerize` and then just start the container and push onto it
- run integration tests using `make test`

## TODOs

Next steps:
- authentication