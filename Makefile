BINARY_NAME=mcp-k8s-networking
IMAGE_NAME=mcp-k8s-networking
IMAGE_TAG?=latest

.PHONY: build test clean docker-build run

build:
	go build -o bin/$(BINARY_NAME) ./cmd/server/

test:
	go test ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

run:
	CLUSTER_NAME=local go run ./cmd/server/

tidy:
	go mod tidy

lint:
	golangci-lint run ./...
