FROM golang:alpine as builder

LABEL Author "Bruce Wallis <bwallis@nokia.com>"

ARG WORK_DIR="/usr/src/kubemgr"
ARG BINARY_NAME="kubemgr"
COPY . $WORK_DIR

WORKDIR $WORK_DIR

ENV HTTP_PROXY $http_proxy
ENV HTTPS_PROXY $https_proxy

RUN apk add --no-cache --virtual build-dependencies build-base=~0.5 && \
    make clean && \
    make build

FROM alpine:3
ARG WORK_DIR="/usr/src/kubemgr"
ARG BINARY_NAME="kubemgr"
RUN mkdir -p /${BINARY_NAME}/bin
RUN mkdir -p /${BINARY_NAME}/yang
COPY --from=builder ${WORK_DIR}/build/$BINARY_NAME /${BINARY_NAME}/bin/
COPY --from=builder ${WORK_DIR}/appmgr/kube.yang /${BINARY_NAME}/yang
COPY --from=builder ${WORK_DIR}/appmgr/kubemgr_config.yml /${BINARY_NAME}/
WORKDIR /

LABEL io.k8s.display-name="Nokia SR Linux Kubernetes Agent"

COPY ./images/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
