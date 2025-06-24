.PHONY: build
build:
	go build -o nix-store-edit .

.PHONY: clean
clean:
	rm -f nix-store-edit

.PHONY: test
test:
	go test ./...

.PHONY: install
install: build
	install -m 755 nix-store-edit $(PREFIX)/bin/

PREFIX ?= /usr/local