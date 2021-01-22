.PHONY: setup clean install generate

setup: clean install generate

clean:
	rm -rf ./tmp

install:
	mkdir -p /tmp
	./scripts/install_protoc.sh
	git clone --depth 1 https://github.com/googleapis/googleapis.git tmp/googleapis

generate:
	go generate ./protobuf/protobuf.go