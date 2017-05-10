all: build

build-daemon:
	CGO_ENABLED=0 go build -v

build: build-daemon

build-static:
	CGO_ENABLED=0 go build -v -a -o deviced

docker: build
	docker build --tag="fuserobotics/deviced:base" .

push: docker
	docker tag fuserobotics/deviced:base registry.fusebot.io/fuserobotics/deviced:base
	docker push registry.fusebot.io/fuserobotics/deviced:base

buildarm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -v -a -o deviced

clean:
	-git clean -Xfd
