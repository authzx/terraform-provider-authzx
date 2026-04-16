default: build

build:
	go build -o terraform-provider-authzx

install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/authzx/authzx/0.1.0/$$(go env GOOS)_$$(go env GOARCH)
	cp terraform-provider-authzx ~/.terraform.d/plugins/registry.terraform.io/authzx/authzx/0.1.0/$$(go env GOOS)_$$(go env GOARCH)/

test:
	go test ./... -v

testacc:
	TF_ACC=1 go test ./... -v -timeout 30m

fmt:
	gofmt -w ./internal ./helpers

fmtcheck:
	@gofmt -l ./internal ./helpers | grep . && exit 1 || true

lint:
	golangci-lint run ./...

docs:
	go generate ./...

clean:
	rm -f terraform-provider-authzx

.PHONY: build install test testacc fmt fmtcheck lint docs clean
