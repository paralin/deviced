all: build

build-daemon:
	go build

build: build-daemon

build-static:
	CGO_ENABLED=0 go build -a -o deviced

docker: build-static
	docker build --tag="synrobo/deviced:latest" .

clean:
	-git clean -Xfd
