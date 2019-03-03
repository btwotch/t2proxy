.PHONY: clean mrproper docker-build

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

docker-build: Dockerfile testclient/Dockerfile docker-compose.yml
	docker-compose build

docker: docker-build
	docker-compose up
