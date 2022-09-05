init:
	docker buildx create --name mybuilder --driver docker-container
	docker buildx use mybuilder
build:
	docker build -t podcompose/agent .