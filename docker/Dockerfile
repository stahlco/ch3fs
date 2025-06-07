FROM golang:latest
WORKDIR /app

COPY . .

RUN go build -o /bin/chefs

CMD ["/bin/chefs"]
