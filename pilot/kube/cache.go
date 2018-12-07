package kube

import (
	"fmt"
	"os"

	"github.com/caicloud/log-pilot/pilot/log"

	"github.com/caicloud/clientset/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Cache interface {
	// Start run informer in another goroutine, and wait for it synced.
	Start(stopCh <-chan struct{}) error
	GetReleaseMeta(namespace, pod string) map[string]string
	GetLegacyLogSources(namespace, pod, container string) []string
}

// New create a new Cache
func New() (Cache, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("NODE_NAME env not defined")
	}
	pc, err := newPodsCache(nodeName, kc)
	if err != nil {
		return nil, err
	}
	return &kubeCache{
		pc: pc,
	}, nil
}

type kubeCache struct {
	pc *podsCache
}

func (c *kubeCache) Start(stopCh <-chan struct{}) error {
	return c.pc.lwCache.Run(stopCh)
}

var (
	releaseAnnotationKeys = map[string]string{
		"helm.sh/namespace": "kubernetes.annotations.helm_sh/namespace",
		"helm.sh/release":   "kubernetes.annotations.helm_sh/release",
	}
	releaseLabelKeys = map[string]string{
		"controller.caicloud.io/chart": "kubernetes.labels.controller_caicloud_io/chart",
	}
)

func releaseMeta(pod *corev1.Pod) map[string]string {
	ret := make(map[string]string)
	if pod == nil {
		return ret
	}
	for k, docKey := range releaseAnnotationKeys {
		v := pod.GetAnnotations()[k]
		if v != "" {
			ret[docKey] = v
		}
	}
	for k, docKey := range releaseLabelKeys {
		v := pod.GetLabels()[k]
		if v != "" {
			ret[docKey] = v
		}
	}
	return ret
}

func (c *kubeCache) GetReleaseMeta(namespace, name string) map[string]string {
	pod, err := c.pc.Get(namespace, name)
	if err != nil {
		log.Errorf("error get pod from cache: %v", err)
		return nil
	}
	return releaseMeta(pod)
}

func (c *kubeCache) GetLegacyLogSources(namespace, podName, containerName string) []string {
	pod, err := c.pc.Get(namespace, podName)
	if err != nil {
		log.Errorf("error get pod from cache: %v", err)
		return nil
	}

	if !requireFileLog(pod) {
		return nil
	}

	sources, err := extractLogSources(pod, containerName)
	if err != nil {
		log.Errorf("error decode log sources from pod annotation: %v", err)
	}
	return sources
}

type podsCache struct {
	lwCache *ListWatchCache
	kc      kubernetes.Interface
}

func newPodsCache(nodeName string, kc kubernetes.Interface) (*podsCache, error) {
	c, e := NewListWatchCache(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fmt.Sprintf("spec.nodeName=%s", nodeName)
			return kc.CoreV1().Pods("").List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fmt.Sprintf("spec.nodeName=%s", nodeName)
			options.Watch = true
			return kc.CoreV1().Pods("").Watch(options)
		},
	}, &corev1.Pod{})
	if e != nil {
		return nil, e
	}
	return &podsCache{
		lwCache: c,
		kc:      kc,
	}, nil
}

func (tc *podsCache) Get(namespace, key string) (*corev1.Pod, error) {
	if obj, exist, e := tc.lwCache.GetInNamespace(namespace, key); exist && obj != nil && e == nil {
		if pod, _ := obj.(*corev1.Pod); pod != nil && pod.Name == key {
			return pod.DeepCopy(), nil
		}
	}
	pod, e := tc.kc.CoreV1().Pods(namespace).Get(key, metav1.GetOptions{})
	if e != nil {
		return nil, e
	}
	return pod, nil
}
func (tc *podsCache) List() ([]corev1.Pod, error) {
	if items := tc.lwCache.List(); len(items) > 0 {
		re := make([]corev1.Pod, 0, len(items))
		for _, obj := range items {
			pod, _ := obj.(*corev1.Pod)
			if pod != nil {
				re = append(re, *pod)
			}
		}
		if len(re) > 0 {
			return re, nil
		}
	}
	podList, e := tc.kc.CoreV1().Pods("").List(metav1.ListOptions{})
	if e != nil {
		return nil, e
	}
	return podList.Items, nil
}
