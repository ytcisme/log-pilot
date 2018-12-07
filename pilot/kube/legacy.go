package kube

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	annotationLogFiles = "logging.caicloud.io/logfiles"
)

type LogFiles struct {
	Files []LogFile `json:"files"`
}

type LogFile struct {
	Container string `json:"container"`
	Source    string `json:"realPath"`
}

func requireFileLog(pod *corev1.Pod) bool {
	if len(pod.Annotations) == 0 {
		return false
	}
	anno, exist := pod.Annotations[annotationLogFiles]
	if !exist || len(anno) == 0 {
		return false
	}
	return true
}

func extractLogSources(pod *corev1.Pod, container string) ([]string, error) {
	items := LogFiles{}
	if err := json.Unmarshal([]byte(pod.Annotations[annotationLogFiles]), &items); err != nil {
		return nil, fmt.Errorf("error decode: %v", err)
	}
	var sources []string
	for _, logFile := range items.Files {
		if container == logFile.Container {
			sources = append(sources, logFile.Source)
		}
	}
	return sources, nil
}
