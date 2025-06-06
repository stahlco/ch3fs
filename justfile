build:
	@go build -o bin/chefs

run: build
	@./bin/chefs

proto:
    protoc --proto_path=./proto \
      --go_out=./proto --go_opt=paths=source_relative \
      --go-grpc_out=./proto --go-grpc_opt=paths=source_relative \
      $(find ./proto -name "*.proto")

clean:
    find ./proto -name "*.pb.go" -type f -delete

up:
