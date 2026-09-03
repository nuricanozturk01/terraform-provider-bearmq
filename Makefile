BINARY   := terraform-provider-bearmq
VERSION  ?= 0.1.0-dev
OSARCH   := $(shell go env GOOS)_$(shell go env GOARCH)
# Local plugin dir Terraform searches for dev_overrides / manual installs.
INSTALL_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/nuricanozturk01/bearmq/$(VERSION)/$(OSARCH)

.PHONY: build install test testacc lint fmt vet tidy docs clean

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

install: build
	mkdir -p "$(INSTALL_DIR)"
	cp $(BINARY) "$(INSTALL_DIR)/$(BINARY)_v$(VERSION)"

test:
	go test ./... -count=1

# Acceptance tests run the full Terraform lifecycle against an in-process fake
# BearMQ (internal/provider/fakebroker_test.go) — no database or broker needed,
# but a `terraform` binary must be on PATH.
testacc:
	TF_ACC=1 go test ./internal/provider/... -count=1 -timeout 30m -v

vet:
	go vet ./...

fmt:
	gofmt -w -s .

tidy:
	go mod tidy

lint:
	golangci-lint run

# Regenerate docs/ from the provider schema + examples/ (Terraform Registry reads docs/).
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name bearmq

clean:
	rm -f $(BINARY)
