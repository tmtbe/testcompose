buildImage:
	docker build -t podcompose/agent .
build:
	CGO_ENABLED=0 go build -o dist/compose ./cmd/compose
install:
	go install ./cmd/compose
all: install buildImage