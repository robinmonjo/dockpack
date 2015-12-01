# dockpack

## Running

To run dockpack, you may pull the last image from the docker-hub (when it will exist), and start it with this command:

````bash
mkdir -p ~/sandbox
export PORT=9999
docker run -e DOCKER_HUB_USERNAME="xxx" -e DOCKER_HUB_PASSWORD="yyy" -v /var/run/docker.sock:/var/run/docker.sock -v ~/sandbox:/sandbox -p $PORT:$PORT robinmonjo/dockpack:1.0
````

This will start a git server listening on $PORT. You can then add it as a remote on one of your project:

````bash
cd my/super/project
git remote add $remote ssh://$hostname:$PORT/packman.git
git push $remote master
````

All git repository and buildpacks cache will be persisted in the sandbox folder on the host.

## Development

You can dockerize the app using `make dockerize` and the just start the container and push onto it

## TODOs

The error checking doesn't work, it always say it fails event if it doesn't

Next steps:
- authentication
- smart taging to have versions + auto cleanup