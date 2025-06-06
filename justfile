build:
	@go build -o bin/lc

run: build
	@./bin/lc

proto:
    protoc --proto_path=./proto \
      --go_out=./proto --go_opt=paths=source_relative \
      --go-grpc_out=./proto --go-grpc_opt=paths=source_relative \
      $(find ./proto -name "*.proto")

clean:
    find ./proto -name "*.pb.go" -type f -delete