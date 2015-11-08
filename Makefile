GOPATH:=`pwd`/vendor:$(GOPATH)
GO:=$(shell which go)
VERSION:=1.0
HARDWARE=$(shell uname -m)

build: vendor
	GOPATH=$(GOPATH) $(GO) build -ldflags="-X main.version=$(VERSION)"

dockerize:
	GOPATH=$(GOPATH) GOOS=linux $(GO) build -ldflags="-X main.version=$(VERSION)"
	docker build -t robinmonjo/dockpack:$(VERSION) .

clean:
	rm -rf ./dockpack ./release ./vendor/pkg

vendor:
	GOPATH=`pwd`/vendor sh vendor.sh
