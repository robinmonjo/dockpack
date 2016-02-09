GOPATH:=`pwd`/vendor:$(GOPATH)
GO:=$(shell which go)
VERSION:=0.2
HARDWARE=$(shell uname -m)
IMAGE_NAME=robinmonjo/dockpack

build: vendor id_rsa
	GOPATH=$(GOPATH) $(GO) build -ldflags="-X main.version=$(VERSION)"

dockerize: id_rsa
	GOPATH=$(GOPATH) GOOS=linux $(GO) build -ldflags="-X main.version=$(VERSION)"
	docker build -t $(IMAGE_NAME):$(VERSION) .
	
id_rsa:
	ssh-keygen -t rsa -b 2048 -C "dockpack@mail.com" -f id_rsa -N ""

clean:
	rm -rf ./dockpack ./release ./vendor/pkg
	
integration: dockerize
	DOCKPACK_IMAGE=$(IMAGE_NAME):$(VERSION) GOPATH=$(GOPATH) bash -c 'cd integration && go test'
	
tests:
	GOPATH=$(GOPATH) bash -c 'cd auth && go test'

vendor:
	GOPATH=`pwd`/vendor sh vendor.sh
