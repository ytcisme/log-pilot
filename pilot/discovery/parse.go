package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caicloud/log-pilot/pilot/configurer"
	"github.com/caicloud/log-pilot/pilot/log"
	"github.com/docker/docker/api/types"
)

type data struct {
	ContainerInfos map[string]string
}

const (
	labelPodName       = "io.kubernetes.pod.name"
	labelPodID         = "io.kubernetes.pod.uid"
	labelPodNamespace  = "io.kubernetes.pod.namespace"
	labelContainerName = "io.kubernetes.container.name"

	tagPodName       = "kubernetes.pod_name"
	tagPodNamespace  = "kubernetes.namespace_name"
	tagContainerName = "kubernetes.container_name"
	tagNodeName      = "node_name"
)

var (
	nodeName = os.Getenv("NODE_NAME")
)

func containerInfos(containerJSON *types.ContainerJSON) map[string]string {
	labels := containerJSON.Config.Labels
	c := make(map[string]string)
	putIfNotEmpty(c, tagPodName, labels[labelPodName])
	putIfNotEmpty(c, tagPodNamespace, labels[labelPodNamespace])
	putIfNotEmpty(c, tagContainerName, labels[labelContainerName])
	putIfNotEmpty(c, tagNodeName, nodeName)
	return c
}

func parseEnvToMap(envs []string) map[string]string {
	ret := map[string]string{}
	for _, s := range envs {
		items := strings.SplitN(s, "=", 2)
		if len(items) == 2 {
			ret[items[0]] = items[1]
		}
	}
	return ret
}

func putIfNotEmpty(store map[string]string, key, value string) {
	if key == "" || value == "" {
		return
	}
	store[key] = value
}

type logOptionsSet map[string]*logOptions

func (ls logOptionsSet) insert(name, opt, v string) {
	if _, exist := ls[name]; !exist {
		ls[name] = &logOptions{
			name:         name,
			format:       configurer.LogFormatPlain,
			inputOptions: make(map[string]string),
		}
	}

	if opt != "" {
		if opt == "format" {
			if v == "json" {
				ls[name].format = configurer.LogFormatJSON
			} else {
				ls[name].format = configurer.LogFormatPlain
			}
			return
		}

		ls[name].inputOptions[opt] = v
	} else {
		ls[name].source = v
	}
}

// logOptions contains options for one log file
type logOptions struct {
	// Name is a unique identifier defined by user.
	name   string
	source string
	format configurer.LogFormat

	// inputOptions defines log collecting options.
	inputOptions map[string]string
	// runtime and user defined tags
	tags map[string]string
}

func parseLogConfigs(d *discovery, info *containerInfo, containerJSON *types.ContainerJSON) ([]*configurer.LogConfig, error) {
	logOptsSet := logOptionsSet{}
	envMap := parseEnvToMap(containerJSON.Config.Env)
	isLogEnvSet := false

	for k, v := range envMap {
		name, opt := parseLogsEnv(d.logPrefixes, k)
		if name == "" && opt == "" {
			continue
		}

		isLogEnvSet = true
		logOptsSet.insert(name, opt, v)
	}

	// Default to collect stdout.
	if _, exist := logOptsSet["stdout"]; !exist {
		logOptsSet["stdout"] = &logOptions{
			name:   "stdout",
			source: "true",
			format: configurer.LogFormatJSON,
		}
	}

	mountsMap := map[string]types.MountPoint{}
	for _, mount := range containerJSON.Mounts {
		mountsMap[mount.Destination] = mount
	}

	// Check legacy log sources
	if !isLogEnvSet && len(info.LegacyLogSources) > 0 {
		log.Debug("add legacy sources:", info.LegacyLogSources)
		for i, source := range info.LegacyLogSources {
			name := fmt.Sprintf("legacy_%v", i)
			logOptsSet[name] = &logOptions{
				name:   name,
				source: source,
			}
		}
	}

	ret := []*configurer.LogConfig{}
	for _, opts := range logOptsSet {
		if opts.name == "" {
			continue
		}
		if opts.name == "stdout" && opts.source != "true" {
			continue
		}
		// Put meta informations into tags.
		opts.tags = containerInfos(containerJSON)
		if opts.name != "stdout" {
			opts.tags["filePath"] = opts.source
		}
		for k, v := range info.ReleaseMeta {
			opts.tags[k] = v
		}
		cfg, err := parseLogConfig(d, d.base, containerJSON, opts, mountsMap)
		if err != nil {
			log.Errorf("error parse log source %s(image %s): %v", opts.source, containerJSON.Image, err)
			continue
		}

		ret = append(ret, cfg)
	}

	return ret, nil
}

func parseLogConfig(d *discovery, base string, containerJSON *types.ContainerJSON, opts *logOptions, mountsMap map[string]types.MountPoint) (*configurer.LogConfig, error) {
	isStdout := opts.name == "stdout"
	if !isStdout && !filepath.IsAbs(opts.source) {
		return nil, fmt.Errorf("expect absolute path")
	}

	// TODO(Tong Cai): ensure the path is limited to RW-layer(emptyDir) of this container(pod)
	var hostPath string
	if isStdout {
		hostPath = fmt.Sprintf("/var/lib/docker/containers/%s/%s-json.log", containerJSON.ID, containerJSON.ID)
	} else {
		hostPath = hostDirOf(opts.source, mountsMap)
		if hostPath == "" {
			return nil, fmt.Errorf("cannot found file %s on host", opts.source)
		}
	}

	ret := &configurer.LogConfig{
		Name:    opts.name,
		Format:  opts.format,
		LogFile: filepath.Join(base, hostPath),
		InOpts:  opts.inputOptions,
		Tags:    opts.tags,
		Stdout:  isStdout,
	}

	return ret, nil
}

/**
场景：
1. 容器一个路径，中间有多级目录对应宿主机不同的目录
2. containerdir对应的目录不是直接挂载的，挂载的是它上级的目录

查找：从containerdir开始查找最近的一层挂载
*/
func hostDirOf(path string, mounts map[string]types.MountPoint) string {
	confPath := path
	for {
		if point, ok := mounts[path]; ok {
			if confPath == path {
				return point.Source
			} else {
				relPath, err := filepath.Rel(path, confPath)
				if err != nil {
					panic(err)
				}
				return fmt.Sprintf("%s/%s", point.Source, relPath)
			}
		}
		path = filepath.Dir(path)
		if path == "/" || path == "." {
			break
		}
	}
	return ""
}

func getMountMap(containerJSON *types.ContainerJSON) map[string]types.MountPoint {
	ret := map[string]types.MountPoint{}
	for _, m := range containerJSON.Mounts {
		ret[m.Destination] = m
	}
	return ret
}

// definitions of multiline_pattern, include_lines, exclude_lines can be found in
// https://github.com/elastic/beats/blob/v6.4.2/filebeat/filebeat.reference.yml
var validOptions = []string{"multiline_pattern", "include_lines", "exclude_lines"}

func parseLogsEnv(prefixes []string, key string) (name, opt string) {
	var (
		prefix string
	)
	for _, pf := range prefixes {
		if strings.HasPrefix(key, pf) {
			prefix = pf
			break
		}
	}
	if prefix == "" {
		return
	}
	s := strings.TrimPrefix(key, prefix)
	for _, o := range validOptions {
		suf := "_" + o
		if strings.HasSuffix(s, suf) {
			if strings.HasSuffix(s, suf) {
				return s[:len(s)-len(suf)], o
			}
		}
	}
	return s, ""
}
