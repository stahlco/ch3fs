FROM golang:alpine AS build

RUN apk add git

WORKDIR /app
COPY . .

RUN go build -o /bin/chefs

FROM alpine:3.19
WORKDIR /app

COPY --from=build /bin/chefs  .

CMD ["./chefs"]