filebeat.config:
  inputs:
    enabled: true
    path: ${path.home}/inputs.d/*.yml
    reload.enabled: true
    reload.period: 10s

filebeat.inputs: 
- type: log
  enabled: true
  fields_under_root: true
  paths:
  - /var/log/dockerd.log
  fields:
    cluster: ${CLUSTER_ID}
    node_name: ${NODE_NAME}
    component: system.docker
- type: log
  enabled: true
  fields_under_root: true
  paths:
  - /var/log/kubelet.log
  fields:
    cluster: ${CLUSTER_ID}
    node_name: ${NODE_NAME}
    component: system.kubelet
# TODO: etcd, apiserver and more..

processors:
- rename:
    fields:
    - from: message
      to: log
    ignore_missing: true
- drop_fields:
    fields: ["beat", "host.name", "input.type", "prospector.type", "offset", "source", ]

{{- if eq .type "elasticsearch" }}
setup.template.enabled: true
setup.template.overwrite: false
setup.template.name: k8s-log-template
setup.template.pattern: logstash-*
setup.template.json.name: k8s-log-template
setup.template.json.path: /etc/filebeat/k8s-log-template.json
setup.template.json.enabled: true

output.elasticsearch:
    hosts:
    {{- range .hosts }}
    - {{ . }}
    {{- end }}
    index: logstash-%{+yyyy.MM.dd}
{{- end }}

{{- if eq .type "kafka" }}
output.kafka:
    enabled: true
    hosts:
    {{- range .brokers }}
    - {{ . }}
    {{- end }}
    topic: {{ .topic }}
    version: {{ .version }}
{{- end -}}
