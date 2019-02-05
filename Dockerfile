# build the project first, then copy only the binary
FROM golang:alpine as builder
WORKDIR /build
RUN apk update && apk add --no-cache git
COPY . /build
RUN (cd /build && go get -d -v ./... && CGO_ENABLED=0 GOOS=linux go build)

# this results in an image the size of the binary (~10 MB)
FROM scratch
COPY --from=builder /build/vote-collector /vote_collector
ENTRYPOINT ["/vote_collector"]

LABEL maintainer="Nathan Marley <nathan@dash.org>"
LABEL description="Dash Trust Elections Vote Collector API"
