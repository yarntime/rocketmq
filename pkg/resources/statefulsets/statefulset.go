/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package statefulsets

import (
	"fmt"
	"github.com/huanwei/rocketmq-operator/pkg/apis/rocketmq/v1alpha1"
	"github.com/huanwei/rocketmq-operator/pkg/constants"
	apps "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strconv"
)

func NewNameSvrStatefulSet(cluster *v1alpha1.BrokerCluster) *apps.StatefulSet {
	containers := []v1.Container{
		nameSvrContainer(cluster),
	}

	labels := map[string]string{
		constants.BrokerClusterLabel: fmt.Sprintf(cluster.Name + `-ns`),
		constants.BrockerClusterName: cluster.Name,
		"Release":                    cluster.Name,
	}
	ssReplicas := int32(cluster.Spec.NameSvrReplica)
	ss := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf(cluster.Name + `-ns`),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    v1alpha1.ClusterCRDResourceKind,
				}),
			},
			Labels: labels,
		},
		Spec: apps.StatefulSetSpec{
			Replicas: &ssReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					//ServiceAccountName: "rocketmq-operator",
					NodeSelector: cluster.Spec.NodeSelector,
					//Affinity:     cluster.Spec.Affinity,
					Containers: containers,
				},
			},
			ServiceName: fmt.Sprintf(cluster.Name + `-ns-svc`),
		},
	}
	if cluster.Spec.NameSvrStorage != nil {
		ss.Spec.VolumeClaimTemplates = []v1.PersistentVolumeClaim{NewPersistentVolumeClaim(cluster.Spec.NameSvrStorage, labels)}
		ss.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/opt/store",
			},
		}
	}
	return ss
}

func NewBrokerStatefulSet(cluster *v1alpha1.BrokerCluster, index int) *apps.StatefulSet {
	containers := []v1.Container{
		brokerContainer(cluster, index),
	}

	brokerRole := constants.BrokerRoleSlave
	if index == 0 {
		brokerRole = constants.BrokerRoleMaster
	}
	labels := map[string]string{
		constants.BrokerClusterLabel: fmt.Sprintf(cluster.Name+`-%d`, index),
		constants.BrokerRoleLabel:    brokerRole,
		"Release":                    cluster.Name,
	}

	ssReplicas := int32(cluster.Spec.MembersPerGroup)
	ss := &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      fmt.Sprintf(cluster.Name+`-%d`, index),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    v1alpha1.ClusterCRDResourceKind,
				}),
			},
			Labels: labels,
		},
		Spec: apps.StatefulSetSpec{
			Replicas: &ssReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					//ServiceAccountName: "rocketmq-operator",
					NodeSelector: cluster.Spec.NodeSelector,
					//Affinity:     cluster.Spec.Affinity,
					Containers: containers,
				},
			},
			ServiceName: fmt.Sprintf(cluster.Name+`-svc-%d`, index),
		},
	}
	if cluster.Spec.BrokerStorage != nil {
		ss.Spec.VolumeClaimTemplates = []v1.PersistentVolumeClaim{NewPersistentVolumeClaim(cluster.Spec.BrokerStorage, labels)}
		ss.Spec.Template.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/opt/store",
			},
		}
	}
	return ss
}

func NewPersistentVolumeClaim(storage *v1alpha1.Storage, labels map[string]string) v1.PersistentVolumeClaim {
	volumeSize, _ := resource.ParseQuantity(storage.DataDiskSize)
	return v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "data",
			Labels: labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			StorageClassName: storage.StorageClass,
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			FastMode: storage.FastMode,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: volumeSize,
				},
			},
		},
	}
}

func nameSvrContainer(cluster *v1alpha1.BrokerCluster) v1.Container {
	return v1.Container{
		Name:            "nameserver",
		ImagePullPolicy: v1.PullPolicy(cluster.Spec.ImagePullPolicy),
		Image:           cluster.Spec.NameSvrImage,
		Ports: []v1.ContainerPort{
			{
				ContainerPort: 9876,
			},
		},
	}
}

func brokerContainer(cluster *v1alpha1.BrokerCluster, index int) v1.Container {
	return v1.Container{
		Name:            "broker",
		ImagePullPolicy: v1.PullPolicy(cluster.Spec.ImagePullPolicy),
		Image:           cluster.Spec.BrokerImage,
		Ports: []v1.ContainerPort{
			{
				ContainerPort: 10909,
			},
			{
				ContainerPort: 10911,
			},
		},
		Env: []v1.EnvVar{
			{
				Name:  "DELETE_WHEN",
				Value: cluster.Spec.Properties["DELETE_WHEN"],
			},
			{
				Name:  "FILE_RESERVED_TIME",
				Value: cluster.Spec.Properties["FILE_RESERVED_TIME"],
			},
			{
				Name:  "ALL_MASTER",
				Value: strconv.FormatBool(cluster.Spec.AllMaster),
			},
			{
				Name:  "BROKER_NAME",
				Value: fmt.Sprintf(`broker-%d`, index),
			},
			{
				Name:  "REPLICATION_MODE",
				Value: cluster.Spec.ReplicationMode,
			},
			{
				Name:  "FLUSH_DISK_TYPE",
				Value: cluster.Spec.Properties["FLUSH_DISK_TYPE"],
			},
			{
				Name:  "NAMESRV_ADDRESS",
				Value: cluster.Spec.NameServers,
			},
			{
				Name:  "CLUSTER_NAME",
				Value: cluster.Name,
			},
		},
		Command: []string{"./brokerStart.sh"},
	}
}
