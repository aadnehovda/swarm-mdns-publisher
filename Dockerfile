FROM golang:1.24-alpine AS build

WORKDIR /src
COPY . .
RUN go mod tidy \
	&& CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test ./... \
	&& CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/swarm-mdns-publisher .

FROM alpine:3.21
COPY --from=build /out/swarm-mdns-publisher /usr/local/bin/swarm-mdns-publisher
ENTRYPOINT ["/usr/local/bin/swarm-mdns-publisher"]
