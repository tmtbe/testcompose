build:
	docker build --target agent -t testmesh/compose-agent .
	docker build --target compose -t testmesh/compose .
install:
	go install ./cmd/compose