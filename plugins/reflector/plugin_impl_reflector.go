// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:generate protoc -I ./model/pod --go_out=plugins=grpc:./model/pod ./model/pod/pod.proto
//go:generate protoc -I ./model/namespace --go_out=plugins=grpc:./model/namespace ./model/namespace/namespace.proto
//go:generate protoc -I ./model/policy --go_out=plugins=grpc:./model/policy ./model/policy/policy.proto

package reflector

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/fields"

	"k8s.io/client-go/kubernetes"
	clientapi_v1 "k8s.io/client-go/pkg/api/v1"
	clientapi_v1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
)

type Plugin struct {
	Deps
	*Config

	stopCh chan struct{}
	wg     sync.WaitGroup

	k8sClientConfig *rest.Config
	k8sClientset    *kubernetes.Clientset

	nsReflector *NamespaceReflector

	k8sPolicyStore      cache.Store
	k8sPolicyController cache.Controller

	k8sPodStore      cache.Store
	k8sPodController cache.Controller
}

type Deps struct {
	local.PluginInfraDeps
	Publish datasync.KeyProtoValWriter
}

type ReflectorDeps struct {
	*Config
	Log          logging.Logger
	K8sClientset *kubernetes.Clientset
	Publish      datasync.KeyProtoValWriter
}

// Config holds the settings for the Reflector.
type Config struct {
	// Path to a kubeconfig file to use for accessing the k8s API.
	Kubeconfig string `default:"" split_words:"false" json:"kubeconfig"`
}

func (plugin *Plugin) Init() error {
	plugin.Log.SetLevel(logging.DebugLevel)
	plugin.stopCh = make(chan struct{})

	if plugin.Config == nil {
		plugin.Config = &Config{}
	}

	found, err := plugin.PluginConfig.GetValue(plugin.Config)
	if err != nil {
		return fmt.Errorf("error loading Reflector configuration file: %s", err)
	} else if found {
		plugin.Log.WithField("filename", plugin.PluginConfig.GetConfigName()).Info(
			"Loaded Reflector configuration file")
	} else {
		plugin.Log.Info("Using default Reflector configuration")
	}

	plugin.k8sClientConfig, err = clientcmd.BuildConfigFromFlags("", plugin.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build kubernetes client config: %s", err)
	}

	plugin.k8sClientset, err = kubernetes.NewForConfig(plugin.k8sClientConfig)
	if err != nil {
		return fmt.Errorf("failed to build kubernetes client: %s", err)
	}

	plugin.nsReflector = &NamespaceReflector{}
	plugin.nsReflector.ReflectorDeps.Log = plugin.Log.NewLogger("-namespace")
	plugin.nsReflector.ReflectorDeps.Log.SetLevel(logging.DebugLevel)
	plugin.nsReflector.ReflectorDeps.Config = plugin.Config
	plugin.nsReflector.ReflectorDeps.K8sClientset = plugin.k8sClientset
	plugin.nsReflector.ReflectorDeps.Publish = plugin.Publish
	err = plugin.nsReflector.Init(plugin.stopCh, &plugin.wg)
	if err != nil {
		plugin.Log.WithField("err", err).Error("Failed to initialize Namespace reflector")
		return err
	}

	//plugin.watchPolicies()
	//plugin.watchPods()

	return nil
}

func (plugin *Plugin) watchPolicies() {
	restClient := plugin.k8sClientset.ExtensionsV1beta1().RESTClient()
	listWatch := cache.NewListWatchFromClient(restClient, "networkpolicies", "", fields.Everything())
	plugin.k8sPolicyStore, plugin.k8sPolicyController = cache.NewInformer(
		listWatch,
		&clientapi_v1beta1.NetworkPolicy{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				policy := obj.(*clientapi_v1beta1.NetworkPolicy)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "policy": policy}).
					Info("Network policy added")
			},
			DeleteFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				policy := obj.(*clientapi_v1beta1.NetworkPolicy)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "policy": policy}).
					Info("Network policy deleted")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldKey, _ := cache.MetaNamespaceKeyFunc(oldObj)
				oldPolicy := oldObj.(*clientapi_v1beta1.NetworkPolicy)
				newKey, _ := cache.MetaNamespaceKeyFunc(newObj)
				newPolicy := newObj.(*clientapi_v1beta1.NetworkPolicy)
				plugin.Log.WithFields(map[string]interface{}{"old-key": oldKey, "old-policy": oldPolicy,
					"new-key": newKey, "new-policy": newPolicy}).Info("Network policy changed")
			},
		},
	)

	go plugin.k8sPolicyController.Run(plugin.stopCh)
	plugin.Log.Debug("Waiting to sync with Kubernetes API (NetworkPolicy)")
	for !plugin.k8sPolicyController.HasSynced() {
	}
	plugin.Log.Debug("Finished syncing with Kubernetes API (NetworkPolicy)")
}

func (plugin *Plugin) watchPods() {
	restClient := plugin.k8sClientset.CoreV1().RESTClient()
	listWatch := cache.NewListWatchFromClient(restClient, "pods", "", fields.Everything())
	plugin.k8sPodStore, plugin.k8sPodController = cache.NewInformer(
		listWatch,
		&clientapi_v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				pod := obj.(*clientapi_v1.Pod)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "pod": pod}).
					Info("Pod added")
			},
			DeleteFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				pod := obj.(*clientapi_v1.Pod)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "pod": pod}).
					Info("Pod deleted")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldKey, _ := cache.MetaNamespaceKeyFunc(oldObj)
				oldPod := oldObj.(*clientapi_v1.Pod)
				newKey, _ := cache.MetaNamespaceKeyFunc(newObj)
				newPod := newObj.(*clientapi_v1.Pod)
				plugin.Log.WithFields(map[string]interface{}{"old-key": oldKey, "old-pod": oldPod,
					"new-key": newKey, "new-pod": newPod}).Info("Pod changed")
			},
		},
	)

	go plugin.k8sPodController.Run(plugin.stopCh)
	plugin.Log.Debug("Waiting to sync with Kubernetes API (Pod)")
	for !plugin.k8sPodController.HasSynced() {
	}
	plugin.Log.Debug("Finished syncing with Kubernetes API (Pod)")
}

func (plugin *Plugin) Close() error {
	close(plugin.stopCh)
	safeclose.CloseAll(plugin.nsReflector)
	plugin.wg.Wait()
	return nil
}
