FROM golang:latest
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o ./chefs ./cmd/ch3fs

CMD ["./chefs"]
