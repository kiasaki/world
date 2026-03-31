BINARY := apenv

.PHONY: build test install fmt clean

build:
	go build -o $(BINARY) .

test:
	go test ./...

fmt:
	gofmt -w main.go main_test.go

install:
	go install .

clean:
	rm -f $(BINARY)
