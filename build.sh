#!/bin/bash
set -e

cd $(dirname "${BASH_SOURCE[0]}")


mkdir -p assets

go get github.com/gopherjs/gopherjs/js
gopherjs build -m -o assets/console.js console/cmd/*.go

cd assets
go get github.com/jteeuwen/go-bindata/...
go-bindata -pkg assets \
	-ignore=.gitignore \
	-ignore=.DS_Store \
	-ignore=assets.go \
	-o assets.go \
	./...
cd ..

go build -o play-server cmd/*.go

if [ "$1" == "run" ]; then
	PATH=$PATH:$GOPATH/src/github.com/tidwall/tile38 ./play-server
fi

