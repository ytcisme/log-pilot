package configurer

import (
	"github.com/caicloud/log-pilot/pilot/container"
)

// Configurer receive events from discovery and manage input configurations
type Configurer interface {
	Name() string
	Start() error
	Stop()
	BootstrapCheck() (map[string]*InputConfigFile, error)
	OnAdd(ev *ContainerAddEvent) error
	OnDestroy(ev *ContainerDestroyEvent) error
}

// ContainerAddEvent contains data for handling container update
type ContainerAddEvent struct {
	Container  container.Container
	LogConfigs []*LogConfig
}

// ContainerDestroyEvent contains data for handling container delete
type ContainerDestroyEvent struct {
	Container container.Container
}

type LogConfig struct {
	Name string
	// LogFile is absolute path of the log file on host.
	LogFile string
	// Format defines log format.
	Format LogFormat
	// Tags are addtional informations that will be added to log record.
	// For example, pod informations, user defined tags.
	Tags   map[string]string
	InOpts map[string]string
	Stdout bool
}

type LogFormat string

const (
	LogFormatJSON  = "json"
	LogFormatPlain = "plain"
)

type InputConfigFile struct {
	Namespace   string
	Pod         string
	Container   string
	ContainerID string
	Version     string
	// Absolute filepath.
	Path string
}
