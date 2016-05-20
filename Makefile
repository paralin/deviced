all: build

build-daemon:
	go build

build: build-daemon

clean:
	-git clean -Xfd
