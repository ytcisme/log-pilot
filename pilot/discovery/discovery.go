package discovery

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/caicloud/log-pilot/pilot/configurer"
	"github.com/caicloud/log-pilot/pilot/kube"
	"github.com/caicloud/log-pilot/pilot/log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/elastic/beats/libbeat/logp"
)

// Discovery watchs container start and destory events,
// and send parsed results to configurer.
type Discovery interface {
	Start(ctx context.Context) error
	Stop()
}

// containerInfo saves basic informations for a container
type containerInfo struct {
	ID            string
	PodID         string
	PodName       string
	ContainerName string
	Namespace     string
	ReleaseMeta   map[string]string
	// Compatible with old interface, which use pod annotation to store
	// log sources.
	LegacyLogSources []string
}

type discovery struct {
	logger          log.Logger
	configurer      configurer.Configurer
	client          *client.Client
	base            string
	logPrefixes     []string
	existContainers map[string]*containerInfo
	cache           kube.Cache
	mutex           sync.Mutex
}

// New creates a new Discovery
func New(baseDir, logPrefix string, configurer configurer.Configurer) (Discovery, error) {
	if os.Getenv("DOCKER_API_VERSION") == "" {
		os.Setenv("DOCKER_API_VERSION", "1.23")
	}

	client, err := client.NewEnvClient()
	if err != nil {
		return nil, fmt.Errorf("error create docker client: %v", err)
	}

	var prefixes []string
	if logPrefix == "" {
		prefixes = []string{"log_"}
	} else {
		for _, each := range strings.Split(logPrefix, ",") {
			prefixes = append(prefixes, each+"_log_")
		}
	}

	logger := logp.NewLogger("discovery")
	logger.Info("Use log prefix:", logPrefix)

	cache, err := kube.New()
	if err != nil {
		return nil, fmt.Errorf("error create pod cache: %v", err)
	}

	return &discovery{
		logger:          logger,
		configurer:      configurer,
		client:          client,
		cache:           cache,
		base:            baseDir,
		logPrefixes:     prefixes,
		existContainers: make(map[string]*containerInfo),
	}, nil
}

// Start runs a work loop
func (d *discovery) Start(ctx context.Context) error {
	d.logger.Info("Start discovery")

	if err := d.cache.Start(ctx.Done()); err != nil {
		return fmt.Errorf("error start pod cache: %v", err)
	}
	d.logger.Info("Cache synced")

	if err := d.configurer.Start(); err != nil {
		return err
	}
	d.logger.Info("Configurer started")

	if err := d.watch(ctx); err != nil {
		return err
	}

	return nil
}

func (d *discovery) watch(ctx context.Context) error {
	startTs := time.Now()
	if err := d.processAllContainers(); err != nil {
		return fmt.Errorf("error process all containers for the first time: %v", err)
	}
	d.logger.Infof("Cost %v to process all events", time.Since(startTs))

	collected, err := d.configurer.GetCollectedContainers()
	if err != nil {
		return fmt.Errorf("error get collected containers: %v", err)
	}

	// Remove configuration files if container not exist
	for ID := range collected {
		if _, exist := d.existContainers[ID]; !exist {
			if err := d.configurer.OnDestroy(&configurer.ContainerDestroyEvent{ID: ID}); err != nil {
				d.logger.Error("error remove container", ID, "from inputs")
			}
		}
	}

	filter := filters.NewArgs()
	filter.Add("type", "container")

	options := types.EventsOptions{
		Filters: filter,
	}
	msgs, errs := d.client.Events(ctx, options)
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("Discovery watch stopped")
			return nil
		default:
		}

		select {
		case msg := <-msgs:
			if err := d.processEvent(msg); err != nil {
				d.logger.Errorf("fail to process event: %v,  %v", msg, err)
			}
		case err := <-errs:
			d.logger.Warnf("error: %v", err)
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			} else {
				msgs, errs = d.client.Events(ctx, options)
			}
		}
	}
}

func (d *discovery) processAllContainers() error {
	opts := types.ContainerListOptions{}
	containers, err := d.client.ContainerList(context.Background(), opts)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if c.State == "removing" {
			continue
		}
		containerJSON, err := d.client.ContainerInspect(context.Background(), c.ID)
		if err != nil {
			return err
		}
		if err = d.newContainer(&containerJSON); err != nil {
			d.logger.Errorf("fail to process container %s: %v", containerJSON.Name, err)
			continue
		}
	}

	return nil
}

func getContainerInfo(cache kube.Cache, containerJSON *types.ContainerJSON) *containerInfo {
	ret := &containerInfo{
		ID: containerJSON.ID,
	}
	if containerJSON.Config.Labels != nil {
		ret.PodID = containerJSON.Config.Labels[labelPodID]
		ret.PodName = containerJSON.Config.Labels[labelPodName]
		ret.Namespace = containerJSON.Config.Labels[labelPodNamespace]
		ret.ContainerName = containerJSON.Config.Labels[labelContainerName]
	}
	if ret.PodName != "" && ret.Namespace != "" {
		ret.ReleaseMeta = cache.GetReleaseMeta(ret.Namespace, ret.PodName)
		ret.LegacyLogSources = cache.GetLegacyLogSources(ret.Namespace, ret.PodName, ret.ContainerName)
	}
	return ret
}

func (d *discovery) addContainer(ID string, info *containerInfo) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.existContainers[ID] = info
}

func (d *discovery) processEvent(msg events.Message) error {
	containerID := msg.Actor.ID
	ctx := context.Background()
	switch msg.Action {
	case "start", "restart":
		d.logger.Infof("Process container start event: %s", containerID)
		if d.exists(containerID) {
			d.logger.Infof("%s is already exists.", containerID)
			return nil
		}
		containerJSON, err := d.client.ContainerInspect(ctx, containerID)
		if err != nil {
			return err
		}
		return d.newContainer(&containerJSON)
	case "destroy":
		d.logger.Infof("Process container destory event: %s", containerID)
		err := d.delContainer(containerID)
		if err != nil {
			d.logger.Warnf("Process container destory event error: %s, %s", containerID, err.Error())
		}
	}
	return nil
}

func (d *discovery) exists(ID string) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	if d.existContainers == nil {
		return false
	}
	_, exist := d.existContainers[ID]
	return exist
}

func (d *discovery) newContainer(containerJSON *types.ContainerJSON) error {
	info := getContainerInfo(d.cache, containerJSON)
	if len(containerJSON.Config.Labels) > 0 {
		// Skip POD containers
		if info.ContainerName == "POD" {
			return nil
		}
	}

	log.Debug("container info:", *info)

	logConfigs, err := parseLogConfigs(d, info, containerJSON)
	if err != nil {
		return err
	}

	if len(logConfigs) == 0 {
		d.logger.Debugf("No log collecting config for container %s", containerJSON.ID)
		return nil
	}

	ev := &configurer.ContainerUpdateEvent{
		ID:         containerJSON.ID,
		LogConfigs: logConfigs,
	}

	if err := d.configurer.OnUpdate(ev); err != nil {
		return fmt.Errorf("error update config: %v", err)
	}

	d.addContainer(containerJSON.ID, info)

	return nil
}

func (d *discovery) delContainer(ID string) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if info, exist := d.existContainers[ID]; exist {
		delete(d.existContainers, ID)
		return d.configurer.OnDestroy(&configurer.ContainerDestroyEvent{
			ID:    info.ID,
			PodID: info.PodID,
		})
	}

	return nil
}

func (d *discovery) Stop() {
}
