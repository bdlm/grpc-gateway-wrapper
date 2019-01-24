########################################################################
# protobuff go compiler
# Author: eo-web-ng@returnpath.com
########################################################################

FROM golang:1.11

# System setup
ENV DEBIAN_FRONTEND=noninteractive \
    TERM=xterm \
    TIMEZONE=UTC \
    NLS_LANG=American_America.AL32UTF8 \
    ORACLE_HOME=/oracle/product/latest

RUN set -x \
    # Basic .bashrc
    && echo 'alias ll="ls -laF"' >> /root/.bashrc \
    && echo 'alias e="exit"' >> /root/.bashrc \
    && echo 'alias cls="clear"' >> /root/.bashrc \
    # System software
    && curl -sL https://deb.nodesource.com/setup_11.x | bash - \
    && apt-get -qqy update \
    && apt-get -qqy --no-install-recommends install \
        autoconf \
        automake \
        build-essential \
        ca-certificates \
        gnupg \
        libcap2-bin \
        libtool \
        nodejs \
        pkg-config \
        tzdata \
        wget \
    && npm_config_user=root npm install -g --save \
        google-protobuf \
        ts-protoc-gen \
    # Timezone config
    && echo $TIMEZONE > /etc/timezone \
    && DEBCONF_NONINTERACTIVE_SEEN=true dpkg-reconfigure --frontend noninteractive tzdata \
    # Clean up tmp and setup files
    && rm -rf \
        /tmp/* \
        /tmp/.build \
        /var/lib/apt/lists/* \
        /var/cache/apt/*

RUN mkdir -p /go/src/github.com \
    && cd /go/src/github.com \
    && git clone -b $(curl -L https://grpc.io/release) https://github.com/grpc/grpc \
    && cd grpc \
    && git submodule update --init --recursive

RUN cd /go/src/github.com/grpc \
    && make grpc_php_plugin

RUN cd /go/src/github.com/grpc \
    && make \
    && make install

RUN cd /go/src/github.com/grpc/third_party/protobuf \
    && make install

RUN go get -u \
    github.com/golang/mock/... \
    github.com/golang/protobuf/proto \
    github.com/golang/protobuf/protoc-gen-go \
    github.com/grpc-ecosystem/grpc-gateway/... \
    github.com/lyft/protoc-gen-validate \
    github.com/shurcooL/vfsgen

COPY . /go/src/github.com/bdlm/generate/
RUN mv /go/src/github.com/bdlm/generate/include/google/protobuf /usr/local/include/google/proto
VOLUME [ "/src" ]
WORKDIR /src

ENTRYPOINT ["/go/src/github.com/bdlm/generate/entrypoint.sh"]
