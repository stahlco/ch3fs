build: proto
	@go build -o bin/chefs

run: build
	@./bin/chefs

proto:
    echo "Generating the .proto files"
    protoc --proto_path=./proto \
      --go_out=./proto --go_opt=paths=source_relative \
      --go-grpc_out=./proto --go-grpc_opt=paths=source_relative \
      $(find ./proto -name "*.proto")
    go mod tidy

clean:
    find ./proto -name "*.pb.go" -type f -delete

# Spins up the containers
up:
   docker compose up --build --detach --timeout 3

# Spins up/ Shuts down
scale x:
   docker compose up --detach --scale ch3f={{x}}

# Shuts down the containers (remove-orphans = Removes any containers created by a previous run, )
down:
    docker compose down --remove-orphans --timeout 3

