FROM --platform=linux/amd64 golang:1.26-bookworm@sha256:9fdc884aacc3bec89b20ffc69f4bb369c78210e3e4f600387b5128b12c199f81
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get -y update && \
    apt-get -y install apt-utils gcc-aarch64-linux-gnu file &&  \
    curl -sSLO https://github.com/go-task/task/releases/download/v3.33.1/task_linux_amd64.deb && \
    apt-get -y install ./task_linux_amd64.deb && \
    rm -rf ./task_linux_amd64.deb /var/cache/apt/* /var/lib/apt/lists/* /var/log/*

ADD cmd /.trx-deps/cmd
ADD internal /.trx-deps/internal
ADD go.mod go.sum Taskfile.dist.yaml /.trx-deps/
ADD scripts /.trx-deps/scripts

RUN cd /.trx-deps && \
    task build:dist version=base && \
    task verify:dist:binaries version=base && \
    rm -rf /.trx-deps

RUN git config --global --add safe.directory /git