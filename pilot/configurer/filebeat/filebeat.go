package filebeat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/caicloud/log-pilot/pilot/configurer"
	"github.com/caicloud/log-pilot/pilot/log"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/go-ucfg"
)

// logStates contains states in filebeat registry and related to the container
type logStates struct {
	// ID is container ID
	ID     string
	PodID  string
	states []RegistryState
	ts     time.Time
}

type filebeatConfigurer struct {
	name string
	base string
	// Filebeat home path.
	filebeatHome   string
	tmpl           *template.Template
	watchDone      chan bool
	watchDuration  time.Duration
	watchContainer map[string]*logStates
	logger         log.Logger
	lock           sync.Mutex
}

// New creates a new filebeat configurer.
func New(baseDir, configTemplateFile, filebeatHome string) (configurer.Configurer, error) {
	t, err := template.ParseFiles(configTemplateFile)
	if err != nil {
		return nil, fmt.Errorf("error parse log template: %v", err)
	}

	if _, err := os.Stat(filebeatHome); err != nil {
		return nil, err
	}

	logger := logp.NewLogger("configurer")
	c := &filebeatConfigurer{
		logger:         logger,
		name:           "filebeat",
		filebeatHome:   filebeatHome,
		base:           baseDir,
		tmpl:           t,
		watchDone:      make(chan bool),
		watchContainer: make(map[string]*logStates, 0),
		watchDuration:  60 * time.Second,
	}

	if err := os.MkdirAll(c.getProspectorsDir(), 0644); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *filebeatConfigurer) Start() error {
	go func() {
		if err := c.watch(); err != nil {
			c.logger.Errorf("error watch: %v", err)
		}
	}()
	return nil
}

var configOpts = []ucfg.Option{
	ucfg.PathSep("."),
	ucfg.ResolveEnv,
	ucfg.VarExp,
}

// FileInode is copied from beats/filebeat/registar/registar.go
type FileInode struct {
	Inode  uint64 `json:"inode,"`
	Device uint64 `json:"device,"`
}

// RegistryState is copied from beats/filebeat/registar/registar.go
type RegistryState struct {
	Source      string        `json:"source"`
	Offset      int64         `json:"offset"`
	Timestamp   time.Time     `json:"timestamp"`
	TTL         time.Duration `json:"ttl"`
	Type        string        `json:"type"`
	FileStateOS FileInode
}

func (c *filebeatConfigurer) getProspectorsDir() string {
	return filepath.Join(c.filebeatHome, "/inputs.d")
}

func (c *filebeatConfigurer) getRegistryFile() string {
	return filepath.Join(c.filebeatHome, "data/registry")
}

func (c *filebeatConfigurer) GetCollectedContainers() (map[string]struct{}, error) {
	ret := make(map[string]struct{})
	files, err := ioutil.ReadDir(c.getProspectorsDir())
	if err != nil {
		return nil, err
	}

	for i := range files {
		if files[i].IsDir() {
			continue
		}
		if !strings.HasSuffix(files[i].Name(), ".yml") {
			continue
		}
		ID := strings.TrimSuffix(files[i].Name(), ".yml")
		ret[ID] = struct{}{}
	}

	return ret, nil
}

func (c *filebeatConfigurer) getContainerConfigPath(containerID string) string {
	confFile := containerID + ".yml"
	return filepath.Join(c.getProspectorsDir(), confFile)
}

func (c *filebeatConfigurer) watch() error {
	c.logger.Infof("%s watcher start", c.Name())
	for {
		select {
		case <-c.watchDone:
			c.logger.Infof("%s watcher stop", c.Name())
			return nil
		case <-time.After(c.watchDuration):
			c.logger.Infof("%s watcher scan", c.Name())

			startTs := time.Now()
			err := c.scan()
			c.logger.Debugf("cost %v to complete scan", time.Since(startTs))
			if err != nil {
				c.logger.Errorf("%s watcher scan error: %v", c.Name(), err)
			}

		}
	}
}

// scan gc for input files
func (c *filebeatConfigurer) scan() error {
	states, err := c.getRegsitryState()
	if err != nil {
		return nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	c.logger.Debugf("watching containers: %#v", c.watchContainer)

	for container, lst := range c.watchContainer {
		confPath := c.getContainerConfigPath(container)
		if _, err := os.Stat(confPath); err != nil && os.IsNotExist(err) {
			c.logger.Infof("log config %s.yml has been removed and ignore", container)
			delete(c.watchContainer, container)
		} else if c.canRemoveConf(container, states, lst) {
			c.logger.Infof("try to remove log config %s.yml", container)
			if err := os.Remove(confPath); err != nil {
				c.logger.Errorf("remove log config %s.yml fail: %v", container, err)
			} else {
				delete(c.watchContainer, container)
			}
		} else {
			c.logger.Debugf("%s.yml cannot be removed for now, will try to remove it in next scan", container)
		}
	}
	return nil
}

var (
	zeroTime time.Time
)

func getLogDirPrefix(base, podID string) string {
	return filepath.Join(base, fmt.Sprintf("/var/lib/kubelet/pods/%s/volumes/kubernetes.io~empty-dir", podID))
}

// 检查已删除容器 input 文件是否可以移除
// 先根据用 pod id 生成唯一路径前缀，然后从 registry file 中找到相应的 states
// 如果是第一次检查，更新 logStates 并返回 false
// 否则和上一次检查对比，如果 states 有变化说明日志还没采集完, 更新 logStates 并返回 false，若没有变化则返回 true
func (c *filebeatConfigurer) canRemoveConf(container string, registry map[string]RegistryState, lst *logStates) bool {
	logDirPrefix := getLogDirPrefix(c.base, lst.PodID)
	c.logger.Debug("LogDir prefix:", logDirPrefix)

	// Find stats belong to the same pod
	var states []RegistryState
	for source, rs := range registry {
		if strings.HasPrefix(source, logDirPrefix) {
			c.logger.Debug("found match state:", source)
			states = append(states, rs)
		}
	}

	// Sort states by source
	bySource := func(i, j int) bool {
		return states[i].Source < states[j].Source
	}
	sort.Slice(states, bySource)

	if zeroTime == lst.ts {
		// First time check
		c.logger.Debugf("check %s.yml for the first time, states: %#v", container, states)
		lst.states = states
		lst.ts = time.Now()

		if len(states) == 0 {
			return true
		}
		return false
	}

	c.logger.Debugf("check %s.yml, old states: %#v, new states: %#v", container, lst.states, states)

	// Check if states changed
	changed := false
	if len(states) != len(lst.states) {
		changed = true
	} else {
		for i := range states {
			if states[i].Source == lst.states[i].Source &&
				states[i].FileStateOS.Device == lst.states[i].FileStateOS.Device &&
				states[i].FileStateOS.Inode == lst.states[i].FileStateOS.Inode &&
				states[i].Offset == lst.states[i].Offset {
				continue
			} else {
				changed = true
				break
			}
		}
	}

	if changed == true {
		// Update states, keep it and wait for next check
		lst.states = states
		lst.ts = time.Now()
		c.logger.Debugf("inputs for container %s cannot be removed for now due to states changed")
		return false
	}

	return true
}

func (c *filebeatConfigurer) OnUpdate(ev *configurer.ContainerUpdateEvent) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	content, err := c.render(ev)
	if err != nil {
		return fmt.Errorf("error render config file: %v", err)
	}

	confPath := c.getContainerConfigPath(ev.ID)
	if err := ioutil.WriteFile(confPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("error write config file: %v", err)
	}

	c.logger.Info("Configuration updated successfully for container", ev.ID)
	return nil
}

func (c *filebeatConfigurer) render(ev *configurer.ContainerUpdateEvent) (string, error) {
	var buf bytes.Buffer
	context := map[string]interface{}{
		"containerId": ev.ID,
		"configList":  ev.LogConfigs,
	}
	if err := c.tmpl.Execute(&buf, context); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (c *filebeatConfigurer) getRegsitryState() (map[string]RegistryState, error) {
	f, err := os.Open(c.getRegistryFile())
	if err != nil {
		return nil, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	states := make([]RegistryState, 0)
	err = decoder.Decode(&states)
	if err != nil {
		return nil, err
	}

	statesMap := make(map[string]RegistryState, 0)
	for _, state := range states {
		if _, ok := statesMap[state.Source]; !ok {
			statesMap[state.Source] = state
		}
	}
	return statesMap, nil
}

func (c *filebeatConfigurer) Name() string {
	return c.name
}

func (c *filebeatConfigurer) OnDestroy(ev *configurer.ContainerDestroyEvent) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if _, ok := c.watchContainer[ev.ID]; !ok {
		c.watchContainer[ev.ID] = &logStates{
			ID:    ev.ID,
			PodID: ev.PodID,
		}
	}
	return nil
}
