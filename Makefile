.PHONY: clean mrproper docker-build all backend

all: backend

backend: *.go
	goimports -l -w .
	go build .
	go vet
	go fmt
	go test -race
	go build -race

clean:
	rm -fv t2proxy

mrproper: clean

docker: docker-build
	docker build -t t2 --build-arg USER_ID=$(shell id -u) --build-arg GROUP_ID=$(shell id -g) .
	docker run --privileged -v $(shell pwd):/t2proxy -it t2
