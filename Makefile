.PHONY: run build build-linux clean test fmt vet check

run:
	go run .

build:
	go build -trimpath -ldflags="-s -w" -o s3-drive .

build-linux:
	GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o s3-drive-linux .

clean:
	rm -f s3-drive s3-drive-linux

test:
	go test ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

check: fmt vet test
