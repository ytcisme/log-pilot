FROM cargo.caicloudprivatetest.com/library/alpine:3.8

# Use aliyun source
RUN echo "http://mirrors.aliyun.com/alpine/v3.8/main" > /etc/apk/repositories
RUN echo "http://mirrors.aliyun.com/alpine/v3.8/community" >> /etc/apk/repositories

RUN apk update && \ 
    apk add wget && \
    apk add bash && \
    rm -rf /var/cache/apk/*

COPY bin/log-pilot /opt/log-pilot/bin/log-pilot
COPY assets/filebeat/filebeat.tpl /opt/log-pilot

WORKDIR /opt/log-pilot
CMD ["/opt/log-pilot/bin/log-pilot"]
