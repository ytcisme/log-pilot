FROM cargo.caicloudprivatetest.com/library/filebeat:6.4.2

USER root
RUN yum install -y gettext && yum clean all
RUN mkdir /etc/filebeat
COPY build/filebeat/k8s-log-template.json /etc/filebeat/k8s-log-template.json
COPY build/filebeat/filebeat.yml.tpl /etc/filebeat/filebeat.yml.tpl
COPY bin/filebeat-keeper /opt/filebeat-keeper/filebeat-keeper

WORKDIR /usr/share/filebeat
CMD ["/opt/filebeat-keeper/filebeat-keeper"]
