GOPATH:=`pwd`/vendor:$(GOPATH)
GO:=$(shell which go)
VERSION:=0.1
HARDWARE=$(shell uname -m)
IMAGE_NAME=robinmonjo/dockpack

build: vendor id_rsa
	GOPATH=$(GOPATH) $(GO) build -ldflags="-X main.version=$(VERSION)"

dockerize: id_rsa
	GOPATH=$(GOPATH) GOOS=linux $(GO) build -ldflags="-X main.version=$(VERSION)"
	docker build -t $(IMAGE_NAME):$(VERSION) .
	
id_rsa:
	ssh-keygen -t rsa -b 4096 -C "dockpack@mail.com" -f id_rsa -N ""

clean:
	rm -rf ./dockpack ./release ./vendor/pkg

vendor:
	GOPATH=`pwd`/vendor sh vendor.sh
