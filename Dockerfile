FROM golang:1.10-alpine as builder

ENV PILOT_DIR /go/src/github.com/caicloud/log-pilot
ARG GOOS=linux
ARG GOARCH=amd64
WORKDIR $PILOT_DIR
COPY . $PILOT_DIR
RUN go build -o /tmp/log-pilot .

FROM alpine:3.8

# Use aliyun source
RUN echo "http://mirrors.aliyun.com/alpine/v3.8/main" > /etc/apk/repositories
RUN echo "http://mirrors.aliyun.com/alpine/v3.8/community" >> /etc/apk/repositories

RUN apk update && \ 
    apk add wget && \
    apk add bash && \
    rm -rf /var/cache/apk/*

COPY --from=builder /tmp/log-pilot /opt/log-pilot/bin/log-pilot
COPY assets/filebeat/filebeat.tpl /opt/log-pilot

WORKDIR /opt/log-pilot
ENTRYPOINT ["/opt/log-pilot/bin/log-pilot"]
