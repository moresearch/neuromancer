DESTDIR :=
PREFIX := /usr/local

.PHONY: all
all:
	go build -o bin/nm ./src

.PHONY: test
test:
	go test ./src/

.PHONY: install
install:
	install -d $(DESTDIR)$(PREFIX)/bin
	install -d $(DESTDIR)$(PREFIX)/share/man/man1
	install -m755 bin/nm $(DESTDIR)$(PREFIX)/bin
	install -m644 nm.1.gz $(DESTDIR)$(PREFIX)/share/man/man1

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/nm
	rm -f $(DESTDIR)$(PREFIX)/share/man/man1/nm.1.gz

.PHONY: assets
assets:
	python3 ./scripts/themegen.py
	./scripts/pack themes/ words/ quotes/ > src/packed.go
	pandoc -s -t man -o - man.md|gzip > nm.1.gz

.PHONY: rel
rel:
	GOOS=darwin GOARCH=amd64 go build -o bin/nm-osx ./src
	GOOS=windows GOARCH=amd64 go build -o bin/nm.exe ./src
	GOOS=linux GOARCH=amd64 go build -o bin/nm-linux ./src
	GOOS=linux GOARCH=arm go build -o bin/nm-linux_arm ./src
	GOOS=linux GOARCH=arm64 go build -o bin/nm-linux_arm64 ./src
