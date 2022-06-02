coverage: test
	go tool cover -html=coverage.out

test: clean format
	go test -race -coverprofile coverage.out ./...

lint: clean
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.46.2 run .

clean:
	@go clean
	@rm -f profile.out
	@rm -f coverage.out
	@rm -f result.html

format:
	go run mvdan.cc/gofumpt@v0.3.1 -l -w .

help:
	@awk '$$1 ~ /^.*:/ {print substr($$1, 0, length($$1)-1)}' Makefile
