GOPATH:=`pwd`/vendor:$(GOPATH)
GO:=$(shell which go)
VERSION:=0.1
HARDWARE=$(shell uname -m)
IMAGE_NAME=robinmonjo/dockpack

build: vendor
	GOPATH=$(GOPATH) $(GO) build -ldflags="-X main.version=$(VERSION)"

dockerize:
	GOPATH=$(GOPATH) GOOS=linux $(GO) build -ldflags="-X main.version=$(VERSION)"
	docker build -t $(IMAGE_NAME):$(VERSION) .

clean:
	rm -rf ./dockpack ./release ./vendor/pkg

vendor:
	GOPATH=`pwd`/vendor sh vendor.sh
