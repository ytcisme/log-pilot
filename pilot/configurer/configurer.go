package configurer

// Configurer receive events from discovery and manage input configurations
type Configurer interface {
	Name() string
	Start() error
	OnDestroy(ev *ContainerDestroyEvent) error
	OnUpdate(ev *ContainerUpdateEvent) error
	GetCollectedContainers() (map[string]struct{}, error)
}

// ContainerUpdateEvent contains data for handling container update
type ContainerUpdateEvent struct {
	LogConfigs []*LogConfig
	ID         string
}

// ContainerDestroyEvent contains data for handling container delete
type ContainerDestroyEvent struct {
	ID    string
	PodID string
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
