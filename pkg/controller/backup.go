/*
Copyright 2018 The aerospike-operator Authors.

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

package controller

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/travelaudience/aerospike-operator/pkg/backuprestore"
	aerospikeclientset "github.com/travelaudience/aerospike-operator/pkg/client/clientset/versioned"
	aerospikeinformers "github.com/travelaudience/aerospike-operator/pkg/client/informers/externalversions"
	aerospikelisters "github.com/travelaudience/aerospike-operator/pkg/client/listers/aerospike/v1alpha2"
)

const (
	// backupControllerDefaultThreadiness is the number of workers the backup
	// controller will use to process items from the queue.
	backupControllerDefaultThreadiness = 2
)

// AerospikeNamespaceBackupController is the controller for AerospikeNamespaceBackup resources
type AerospikeNamespaceBackupController struct {
	*genericController
	aerospikeNamespaceBackupLister aerospikelisters.AerospikeNamespaceBackupLister
	handler                        *backuprestore.AerospikeBackupRestoreHandler
}

// NewAerospikeNamespaceBackupController returns a new controller for AerospikeNamespaceBackup resources
func NewAerospikeNamespaceBackupController(
	kubeClient kubernetes.Interface,
	aerospikeClient aerospikeclientset.Interface,
	kubeInformerFactory informers.SharedInformerFactory,
	aerospikeInformerFactory aerospikeinformers.SharedInformerFactory) *AerospikeNamespaceBackupController {

	// obtain references to shared informers for the required types
	jobInformer := kubeInformerFactory.Batch().V1().Jobs()
	aerospikeClusterInformer := aerospikeInformerFactory.Aerospike().V1alpha2().AerospikeClusters()
	aerospikeNamespaceBackupInformer := aerospikeInformerFactory.Aerospike().V1alpha2().AerospikeNamespaceBackups()

	// obtain references to listers for the required types
	jobsLister := jobInformer.Lister()
	aerospikeClustersLister := aerospikeClusterInformer.Lister()
	aerospikeNamespaceBackupLister := aerospikeNamespaceBackupInformer.Lister()

	c := &AerospikeNamespaceBackupController{
		genericController:              newGenericController("aerospikenamespacebackup", backupControllerDefaultThreadiness, kubeClient),
		aerospikeNamespaceBackupLister: aerospikeNamespaceBackupLister,
	}
	c.hasSyncedFuncs = []cache.InformerSynced{
		aerospikeNamespaceBackupInformer.Informer().HasSynced,
	}
	c.syncHandler = c.processQueueItem

	c.handler = backuprestore.New(kubeClient, aerospikeClient, aerospikeClustersLister, jobsLister, c.recorder)
	c.logger.Debug("setting up event handlers")

	// setup an event handler for when AerospikeNamespaceBackup resources change
	aerospikeNamespaceBackupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueue,
		UpdateFunc: func(_, obj interface{}) {
			c.enqueue(obj)
		},
	})
	// setup an event handler for when Job resources change. This
	// handler will lookup the owner of the given Job, and if it is
	// owned by a AerospikeNamespaceBackup resource will enqueue that
	// AerospikeNamespaceBackup resource for processing. This way, we don't need
	// to implement custom logic for handling Job resources. More info on this
	// pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*batchv1.Job)
			oldJob := old.(*batchv1.Job)
			if newJob.ResourceVersion == oldJob.ResourceVersion {
				// Periodic resync will send update events for all known Jobs.
				// Two different versions of the same Job will always have different RVs.
				return
			}
			c.handleObject(new)
		},
	})

	return c
}

// processQueueItem compares the actual state with the desired, and attempts to converge the two
func (c *AerospikeNamespaceBackupController) processQueueItem(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the AerospikeNamespaceBackup resource with this namespace/name
	aerospikeNamespaceBackup, err := c.aerospikeNamespaceBackupLister.AerospikeNamespaceBackups(namespace).Get(name)
	if err != nil {
		// The AerospikeNamespaceBackup resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("aerospikenamespacebackup '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// deepcopy aerospikeNamespaceRestore before handle it so we don't possibly mutate the cache
	return c.handler.Handle(aerospikeNamespaceBackup.DeepCopy())
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the AerospikeNamespaceBackup resource that 'owns' it. It does this by
// looking at the objects metadata.ownerReferences field for an appropriate
// OwnerReference. It then enqueues that AerospikeNamespaceBackup resource to be
// processed. If the object does not have an appropriate OwnerReference, it will
// simply be skipped.
func (c *AerospikeNamespaceBackupController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		c.logger.Debugf("recovered deleted object '%s' from tombstone", object.GetName())
	}
	c.logger.Debugf("processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a AerospikeNamespaceBackup, we should
		// not do anything more with it.
		if ownerRef.Kind != "AerospikeNamespaceBackup" {
			return
		}

		asnb, err := c.aerospikeNamespaceBackupLister.AerospikeNamespaceBackups(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			c.logger.Debugf("ignoring orphaned object '%s' of aerospikenamespacebackup '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueue(asnb)
		return
	}
}
