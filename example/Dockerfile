FROM golang:1.11

LABEL org.label-schema.schema-version = 1.0 \
    org.label-schema.vendor = eo-web-ng@returnpath.com \
    org.label-schema.vcs-url = https://github.com/bdlm/grpc-gateway-wrapper \
    org.label-schema.description = "rando" \
    org.label-schema.name = "grpc-gateway-wrapper" \
    org.label-schema.url = https://github.com/bdlm/grpc-gateway-wrapper

ENV TERM=xterm

RUN apt-get update -qqy \
    && apt-get install -qqy \
        telnet \
        vim

VOLUME /go/src/github.com/bdlm/grpc-gateway-wrapper
WORKDIR /go/src/github.com/bdlm/grpc-gateway-wrapper
EXPOSE 80
ENV DEBIAN_FRONTEND=noninteractive \
    TERM=xterm

# Compile and install
COPY . /go/src/github.com/bdlm/grpc-gateway-wrapper/example
RUN cd /go/src/github.com/bdlm/grpc-gateway-wrapper/example \
    && go build -buildmode=pie -o /go/bin/grpc-gateway-wrapper

ENTRYPOINT ["/go/bin/grpc-gateway-wrapper"]
