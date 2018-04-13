/*
Copyright 2018 The aerospike-controller Authors.

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

package framework

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/travelaudience/aerospike-operator/pkg/meta"
	"github.com/travelaudience/aerospike-operator/pkg/utils/listoptions"
)

var (
	// OperatorImage is the image used to deploy aerospike-operator.
	OperatorImage string
	// OperatorNamespace is the namespace in which to crreate the aerospike-operator pod.
	OperatorNamespace string
)

const (
	watchTimeout = 2 * time.Minute

	containerName      = "aerospike-operator"
	nameLabel          = "name"
	operatorCmd        = "/usr/local/bin/aerospike-operator"
	serviceAccountName = "aerospike-operator"
)

func (tf *TestFramework) createOperator() error {
	if OperatorImage == "" {
		log.Warnf("no aerospike-operator image specified, assuming a local instance")
		return nil
	}

	res, err := tf.KubeClient.CoreV1().Pods(OperatorNamespace).Create(createPodObj())
	if err != nil {
		return err
	}
	tf.podName = res.Name

	w, err := tf.KubeClient.CoreV1().Pods(OperatorNamespace).Watch(listoptions.ObjectByName(tf.podName))
	if err != nil {
		return err
	}
	last, err := watch.Until(watchTimeout, w, func(event watch.Event) (bool, error) {
		return event.Object.(*v1.Pod).Status.Phase == v1.PodRunning, nil
	})
	if err != nil {
		return err
	}
	if last == nil {
		return fmt.Errorf("no events received for %s", meta.Key(res))
	}

	return nil
}

func (tf *TestFramework) deleteOperator() error {
	if OperatorImage == "" {
		return nil
	}
	return tf.KubeClient.CoreV1().Pods(OperatorNamespace).Delete(tf.podName, &metav1.DeleteOptions{})
}

func createPodObj() *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: randomNamespacePrefix,
			Labels: map[string]string{
				nameLabel: containerName,
			},
			Namespace: OperatorNamespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            containerName,
					Image:           OperatorImage,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{operatorCmd},
				},
			},
			RestartPolicy:      v1.RestartPolicyNever,
			ServiceAccountName: serviceAccountName,
		},
	}
}
