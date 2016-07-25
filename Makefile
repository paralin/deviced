all: build

build-daemon:
	CGO_ENABLED=0 go build -v

build: build-daemon

build-static:
	CGO_ENABLED=0 go build -v -a -o deviced

docker: build
	docker build --tag="synrobo/deviced:base" .

buildarm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -v -a -o deviced

clean:
	-git clean -Xfd
