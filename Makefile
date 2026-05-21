.PHONY: run build build-linux clean test fmt vet check

# Local dev server. Sessions reset on each restart (ephemeral SESSION_KEY).
run:
	go run .

# Build for the host OS.
build:
	go build -trimpath -ldflags="-s -w" -o s3-drive .

# Cross-compile for the deploy target.
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

# Run before committing.
check: fmt vet test
