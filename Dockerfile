#!/usr/bin/env -S docker build . --tag=schedder-api:latest --network=host --file
FROM docker.io/library/golang:alpine

# Download the dependencies

RUN mkdir -p /tmp/code
COPY go.mod /tmp/code
COPY go.sum /tmp/code
WORKDIR /tmp/code
RUN go mod download
WORKDIR /
RUN rm -rf /tmp/code

# Build & install the backend
COPY . /code
WORKDIR /code
RUN go install ./cmd/schedder-api
RUN rm -rf /code

# Forward the port (NOTE: you still need -p host_port:2023 when running)
EXPOSE 2023
# Run the backend by default when running this container
CMD /go/bin/schedder-api

