{{- range .configList -}}
- type: log
  enabled: true
  paths:
      - {{ .LogFile }}
  scan_frequency: 10s
  fields_under_root: true
  {{- if .Stdout}}
  docker-json: true
  {{end}}
  {{- if eq .Format "json"}}
  json.keys_under_root: true
  {{end}}
  {{- if ( (len .Tags) or (len .InOpts) or (len .OutOpts) ) }}
  fields:
      {{- range $key, $value := .Tags}}
      {{ $key }}: {{ $value }}
      {{end}}
      {{- range $key, $value := .InOpts}}
      {{ $key }}: {{ $value }}
      {{end}}
      {{- range $key, $value := .OutOpts}}
      {{ $key }}: {{ $value }}
      {{end}}
  {{- end -}}
  tail_files: false
  close_inactive: 2h
  close_eof: false
  close_removed: true
  clean_removed: true
  close_renamed: false
{{- end -}}
