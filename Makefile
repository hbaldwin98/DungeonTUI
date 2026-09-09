.PHONY: test vet build install

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -o bin/dungeon ./cmd/dungeon
	go build -o bin/dungeon-harness ./cmd/dungeon-harness
	go build -o bin/dungeon-seed ./cmd/dungeon-seed

install:
	go install ./cmd/dungeon
