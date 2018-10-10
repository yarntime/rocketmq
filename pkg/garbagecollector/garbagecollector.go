package garbagecollector

import (
	"fmt"
	"path"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	rclientset "github.com/huanwei/rocketmq-operator/pkg/generated/clientset/versioned"
	rinformers "github.com/huanwei/rocketmq-operator/pkg/generated/informers/externalversions"
	rlisters "github.com/huanwei/rocketmq-operator/pkg/generated/listers/rocketmq/v1alpha1"
	"github.com/huanwei/rocketmq-operator/pkg/constants"
)

const (
	// Interval represent the interval to run Garabge Collection
	Interval time.Duration = 5 * time.Second
)

// Interface GarbageCollector interface
type Interface interface {
	CollectBrokerClusterGarbage() error
	InformerSync() cache.InformerSynced
}

var _ Interface = &GarbageCollector{}

// GarbageCollector represents a Workflow Garbage Collector.
// It collects orphaned Jobs
type GarbageCollector struct {
	kubeClient kclientset.Interface
	rcClient   rclientset.Interface
	rcLister   rlisters.BrokerClusterLister
	rcSynced   cache.InformerSynced
}

// NewGarbageCollector builds initializes and returns a GarbageCollector
func NewGarbageCollector(rcClient rclientset.Interface, kubeClient kclientset.Interface, rcInformerFactory rinformers.SharedInformerFactory) *GarbageCollector {
	return &GarbageCollector{
		kubeClient: kubeClient,
		rcClient:   rcClient,
		rcLister:   rcInformerFactory.ROCKETMQ().V1alpha1().BrokerClusters().Lister(),
		rcSynced:   rcInformerFactory.ROCKETMQ().V1alpha1().BrokerClusters().Informer().HasSynced,
	}
}

// InformerSync returns the brokercluster cache informer sync function
func (c *GarbageCollector) InformerSync() cache.InformerSynced {
	return c.rcSynced
}

func (c *GarbageCollector) CollectBrokerClusterGarbage() error {
	errs := []error{}
	if err := c.collectBrokerClusterStatefulSets(); err != nil {
		errs = append(errs, err)
	}
	if err := c.collectBrokerClusterServices(); err != nil {
		errs = append(errs, err)
	}
	return utilerrors.NewAggregate(errs)
}

func (c *GarbageCollector) collectBrokerClusterPods() error {
	glog.V(4).Infof("Collecting garbage pods")
	pods, err := c.kubeClient.CoreV1().Pods(metav1.NamespaceAll).List(metav1.ListOptions{
		LabelSelector: constants.BrockerClusterName,
	})
	if err != nil {
		return fmt.Errorf("unable to list brokercluster pods to be collected: %v", err)
	}
	errs := []error{}
	collected := make(map[string]struct{})
	for _, pod := range pods.Items {
		brokerclusterName, found := pod.Labels[constants.BrockerClusterName]
		if !found || len(brokerclusterName) == 0 {
			errs = append(errs, fmt.Errorf("Unable to find brokercluster name for pod: %s/%s", pod.Namespace, pod.Name))
			continue
		}
		if _, done := collected[path.Join(pod.Namespace, brokerclusterName)]; done {
			continue // already collected so skip
		}
		if _, err := c.rcLister.BrokerClusters(pod.Namespace).Get(brokerclusterName); err == nil || !apierrors.IsNotFound(err) {
			if err != nil {
				errs = append(errs, fmt.Errorf("Unexpected error retrieving brokercluster %s/%s cache: %v", pod.Namespace, brokerclusterName, err))
			}
			continue
		}
		// brokerCluster couldn't be find in cache. Trying to get it via APIs.
		if _, err := c.rcClient.ROCKETMQV1alpha1().BrokerClusters(pod.Namespace).Get(brokerclusterName, metav1.GetOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("Unexpected error retrieving brokercluster %s/%s for pod %s/%s: %v", pod.Namespace, brokerclusterName, pod.Namespace, pod.Name, err))
				continue
			}
			// NotFound error: Hence remove all the pods.
			if err := c.kubeClient.CoreV1().Pods(pod.Namespace).DeleteCollection(CascadeDeleteOptions(0), metav1.ListOptions{
				LabelSelector: constants.BrockerClusterName + "=" + brokerclusterName}); err != nil {
				errs = append(errs, fmt.Errorf("Unable to delete Collection of pods for brokercluster %s/%s", pod.Namespace, brokerclusterName))
				continue
			}
			collected[path.Join(pod.Namespace, brokerclusterName)] = struct{}{} // inserted in the collected map
			glog.Infof("Removed all pods for brokercluster %s/%s", pod.Namespace, brokerclusterName)
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (c *GarbageCollector) collectBrokerClusterServices() error {
	glog.V(4).Infof("Collecting garbage services")
	services, err := c.kubeClient.CoreV1().Services(metav1.NamespaceAll).List(metav1.ListOptions{
		LabelSelector: constants.BrockerClusterName,
	})
	if err != nil {
		return fmt.Errorf("unable to list brokercluster services to be collected: %v", err)
	}
	errs := []error{}
	collected := make(map[string]struct{})
	for _, service := range services.Items {
		brokerclusterName, found := service.Labels[constants.BrockerClusterName]
		if !found || len(brokerclusterName) == 0 {
			errs = append(errs, fmt.Errorf("Unable to find brokercluster name for service: %s/%s", service.Namespace, service.Name))
			continue
		}
		if _, done := collected[path.Join(service.Namespace, brokerclusterName)]; done {
			continue // already collected so skip
		}
		if _, err := c.rcLister.BrokerClusters(service.Namespace).Get(brokerclusterName); err == nil || !apierrors.IsNotFound(err) {
			if err != nil {
				errs = append(errs, fmt.Errorf("Unexpected error retrieving brokercluster %s/%s cache: %v", service.Namespace, brokerclusterName, err))
			}
			continue
		}
		// brokerCluster couldn't be find in cache. Trying to get it via APIs.
		if _, err := c.rcClient.ROCKETMQV1alpha1().BrokerClusters(service.Namespace).Get(brokerclusterName, metav1.GetOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("Unexpected error retrieving brokercluster %s/%s for service %s/%s: %v", service.Namespace, brokerclusterName, service.Namespace, service.Name, err))
				continue
			}
			// NotFound error: Hence remove all the pods.
			if err := c.kubeClient.CoreV1().Services(service.Namespace).Delete(service.Name, metav1.NewDeleteOptions(1)); err != nil {
				errs = append(errs, fmt.Errorf("Unable to delete Collection of services for brokercluster %s/%s caused by: %s", service.Namespace, brokerclusterName, err.Error()))
				continue
			}
			collected[path.Join(service.Namespace, brokerclusterName)] = struct{}{} // inserted in the collected map
			glog.Infof("Removed all services for brokercluster %s/%s", service.Namespace, brokerclusterName)
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (c *GarbageCollector) collectBrokerClusterStatefulSets() error {
	glog.V(4).Infof("Collecting garbage statefulset")
	statefulSets, err := c.kubeClient.AppsV1beta1().StatefulSets(metav1.NamespaceAll).List(metav1.ListOptions{
		LabelSelector: constants.BrockerClusterName,
	})
	if err != nil {
		return fmt.Errorf("unable to list brokercluster statefulset to be collected: %v", err)
	}
	errs := []error{}
	collected := make(map[string]struct{})
	for _, set := range statefulSets.Items {
		brokerclusterName, found := set.Labels[constants.BrockerClusterName]
		if !found || len(brokerclusterName) == 0 {
			errs = append(errs, fmt.Errorf("Unable to find statefulset name for statefulset: %s/%s", set.Namespace, set.Name))
			continue
		}
		if _, done := collected[path.Join(set.Namespace, brokerclusterName)]; done {
			continue // already collected so skip
		}
		if _, err := c.rcLister.BrokerClusters(set.Namespace).Get(brokerclusterName); err == nil || !apierrors.IsNotFound(err) {
			if err != nil {
				errs = append(errs, fmt.Errorf("Unexpected error retrieving brokercluster %s/%s cache: %v", set.Namespace, brokerclusterName, err))
			}
			continue
		}
		// brokerCluster couldn't be find in cache. Trying to get it via APIs.
		if _, err := c.rcClient.ROCKETMQV1alpha1().BrokerClusters(set.Namespace).Get(brokerclusterName, metav1.GetOptions{}); err != nil {
			if !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("Unexpected error retrieving brokercluster %s/%s for statefulset %s/%s: %v", set.Namespace, brokerclusterName, set.Namespace, set.Name, err))
				continue
			}
			// NotFound error: Hence remove all the statefulsets.
			if err := c.kubeClient.AppsV1beta1().StatefulSets(set.Namespace).DeleteCollection(CascadeDeleteOptions(0), metav1.ListOptions{
				LabelSelector: constants.BrockerClusterName + "=" + brokerclusterName}); err != nil {
				errs = append(errs, fmt.Errorf("Unable to delete Collection of statefulset for brokercluster %s/%s", set.Namespace, brokerclusterName))
				continue
			}
			collected[path.Join(set.Namespace, brokerclusterName)] = struct{}{} // inserted in the collected map
			glog.Infof("Removed all statefulset for brokercluster %s/%s", set.Namespace, brokerclusterName)
		}
	}
	return utilerrors.NewAggregate(errs)
}

// CascadeDeleteOptions returns a DeleteOptions with Cascaded set
func CascadeDeleteOptions(gracePeriodSeconds int64) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{
		GracePeriodSeconds: func(t int64) *int64 { return &t }(gracePeriodSeconds),
		PropagationPolicy: func() *metav1.DeletionPropagation {
			background := metav1.DeletePropagationBackground
			return &background
		}(),
	}
}
