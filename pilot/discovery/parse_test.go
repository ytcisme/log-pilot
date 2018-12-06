package discovery

import (
	"testing"
)

func TestParseLogEnv(t *testing.T) {
	cases := []struct {
		key  string
		name string
		opt  string
	}{
		{
			"sn_log_foo_bar",
			"foo_bar",
			"",
		},
		{
			"sn_log_foo_bar_filter",
			"foo_bar",
			"filter",
		},
		{
			"aaaa",
			"",
			"",
		},
	}

	for _, cas := range cases {
		name, opt := parseLogsEnv([]string{"sn_log_"}, cas.key)
		if name != cas.name {
			t.Errorf("expect name %s, got %s", cas.name, name)
		}
		if opt != cas.opt {
			t.Errorf("expect opt %s, got %s", cas.opt, opt)
		}
	}
}
