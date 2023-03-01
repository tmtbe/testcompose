buildImage:
	docker build --target agent -t podcompose/agent .
	docker build --target compose -t podcompose/compose .
build:
	CGO_ENABLED=0 go build -o dist/compose ./cmd/compose
install:
	go install ./cmd/compose
all: install buildImage