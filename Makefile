
IMAGE_NAME=generic-sample
IMAGE_TAG=$(shell git rev-parse --short=8 HEAD)

build:
	GOOS=linux go build -a -o genericservice main.go

run:
	go run main.go

docker-build:
	docker build -t $(IMAGE_NAME):${IMAGE_TAG} -f deploy/Dockerfile .
	docker tag $(IMAGE_NAME):${IMAGE_TAG} $(IMAGE_NAME):latest

docker-run: docker-build
	docker run -p9000:9000 -it $(IMAGE_NAME):${IMAGE_TAG}
