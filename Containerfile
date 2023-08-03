FROM docker.io/library/golang:latest AS build
MAINTAINER Simon de Vlieger <cmdr@supakeen.com>

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./

RUN go build -o /buienradar-mqtt

FROM docker.io/library/alpine:latest
MAINTAINER Simon de Vlieger <cmdr@supakeen.com>

COPY --from=build /buienradar-mqtt /buienradar-mqtt

CMD ["/buienradar-mqtt"]
