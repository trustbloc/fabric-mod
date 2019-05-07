# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
ARG GO_VER
ARG ALPINE_VER
FROM alpine:${ALPINE_VER} as base
RUN apk add --no-cache tzdata libltdl

FROM golang:${GO_VER}-alpine${ALPINE_VER} as golang
RUN apk add --no-cache \
	gcc \
	musl-dev \
	git \
	libtool \
	bash \
	make;
ADD . $GOPATH/src/github.com/hyperledger/fabric
WORKDIR $GOPATH/src/github.com/hyperledger/fabric
ENV EXECUTABLES go git
ENV GO111MODULE on
FROM golang as orderer
ARG GO_TAGS
ARG GOPROXY
RUN GO_TAGS=${GO_TAGS} GOPROXY=${GOPROXY} make orderer

FROM base
ENV FABRIC_CFG_PATH /etc/hyperledger/fabric
VOLUME /etc/hyperledger/fabric
VOLUME /var/hyperledger
COPY --from=orderer /go/src/github.com/hyperledger/fabric/.build/bin /usr/local/bin
COPY --from=orderer /go/src/github.com/hyperledger/fabric/sampleconfig/msp ${FABRIC_CFG_PATH}/msp
COPY --from=orderer /go/src/github.com/hyperledger/fabric/sampleconfig/orderer.yaml ${FABRIC_CFG_PATH}
COPY --from=orderer /go/src/github.com/hyperledger/fabric/sampleconfig/configtx.yaml ${FABRIC_CFG_PATH}
EXPOSE 7050
CMD ["orderer"]
