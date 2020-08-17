#FROM golang:1.14-alpine as build
#
#WORKDIR apps/iotex-core
#
#RUN apk add --no-cache make gcc musl-dev linux-headers git
#
#COPY go.mod .
#COPY go.sum .
#ENV GO111MODULE=on
#ENV http_proxy=socks5://192.168.1.8:1080
#RUN git config --global http.proxy 'socks5://192.168.1.8:1080'
#RUN git config --global https.proxy 'socks5://192.168.1.8:1080'
#
#RUN go mod download
#
#COPY . .
#
#RUN mkdir -p $GOPATH/pkg/linux_amd64/github.com/iotexproject/ && \
#    make clean build-all

FROM ubuntu:latest

#RUN apk add --no-cache ca-certificates
#RUN mkdir -p /etc/iotex/
#COPY --from=build /go/apps/iotex-core/bin/server /usr/local/bin/iotex-server
#COPY --from=build /go/apps/iotex-core/bin/actioninjectorv2 /usr/local/bin/iotex-actioninjectorv2
#COPY --from=build /go/apps/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen
#COPY --from=build /go/apps/iotex-core/bin/ioctl /usr/local/bin/ioctl
COPY ./iotex-server /usr/local/bin/iotex-server
CMD [ "iotex-server"]
