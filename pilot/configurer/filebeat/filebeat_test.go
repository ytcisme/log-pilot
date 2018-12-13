package filebeat

import (
	"testing"
	"text/template"

	"github.com/caicloud/log-pilot/pilot/container"

	"github.com/caicloud/log-pilot/pilot/configurer"
)

var (
	expectRenderResult = `
- type: log
  enabled: true
  paths:
      - /opt/tomcat/access.log
  scan_frequency: 10s
  fields_under_root: true
  fields:
      foo: bar
      tail_files: false
  close_inactive: 2h
  close_eof: false
  close_removed: true
  clean_removed: true
  close_renamed: false`
)

func TestRender(t *testing.T) {
	tmpl, err := template.ParseFiles("filebeat.tpl")
	if err != nil {
		t.Fatal(err)
	}

	c := &filebeatConfigurer{
		tmpl: tmpl,
	}
	ev := configurer.ContainerAddEvent{
		Container: container.Container{
			ID: "1",
		},
		LogConfigs: []*configurer.LogConfig{
			&configurer.LogConfig{
				Name:    "access",
				LogFile: "/opt/tomcat/access.log",
				Format:  configurer.LogFormatPlain,
				Tags:    map[string]string{"foo": "bar"},
			},
		},
	}
	_, err = c.render(&ev)
	if err != nil {
		t.Fatal(err)
	}
}
