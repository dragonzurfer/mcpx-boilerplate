build:
	go mod tidy
	go mod vendor
	go build ./...

test:
	go test ./... -coverprofile=coverage.out

fmt:
	gofmt -s -w .
