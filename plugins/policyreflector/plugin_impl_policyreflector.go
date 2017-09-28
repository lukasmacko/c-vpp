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

package policyreflector

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	clientapi "k8s.io/client-go/pkg/api"
	clientapi_v1 "k8s.io/client-go/pkg/api/v1"
	clientapi_v1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/flavors/local"
	"github.com/ligato/cn-infra/utils/safeclose"
)

type Plugin struct {
	Deps
	*Config

	stopCh chan struct{}

	k8sRestClientConfig *rest.Config
	k8sRestClient       *rest.RESTClient

	k8sPolicyStore      cache.Store
	k8sPolicyController cache.Controller

	k8sPodStore      cache.Store
	k8sPodController cache.Controller

	k8sNamespaceStore      cache.Store
	k8sNamespaceController cache.Controller
}

type Deps struct {
	local.PluginInfraDeps
	Publish datasync.KeyProtoValWriter
	Watch   datasync.KeyValProtoWatcher
}

// Config holds the settings for PolicyReflector.
type Config struct {
	// Path to a kubeconfig file to use for accessing the k8s API.
	Kubeconfig string `default:"" split_words:"false" json:"kubeconfig"`
}

func (plugin *Plugin) Init() error {
	plugin.stopCh = make(chan struct{})

	if plugin.Config == nil {
		plugin.Config = &Config{}
	}

	found, err := plugin.PluginConfig.GetValue(plugin.Config)
	if err != nil {
		return fmt.Errorf("error loading PolicyReflector configuration file: %s", err)
	} else if found {
		plugin.Log.WithField("filename", plugin.PluginConfig.GetConfigName()).Info(
			"Loaded PolicyReflector configuration file")
	} else {
		plugin.Log.Info("Using default PolicyReflector configuration")
	}

	// Now build the Kubernetes client, we support in-cluster config and kubeconfig
	// as means of configuring the client.
	plugin.k8sRestClientConfig, err = clientcmd.BuildConfigFromFlags("", plugin.Config.Kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to build kubernetes client config: %s", err)
	}

	// Get extensions client
	plugin.k8sRestClientConfig.GroupVersion = &schema.GroupVersion{
		Group:   "extensions",
		Version: "v1beta1",
	}
	plugin.k8sRestClientConfig.APIPath = "/apis"
	plugin.k8sRestClientConfig.ContentType = runtime.ContentTypeJSON
	plugin.k8sRestClientConfig.NegotiatedSerializer =
		serializer.DirectCodecFactory{CodecFactory: clientapi.Codecs}

	plugin.k8sRestClient, err = rest.RESTClientFor(plugin.k8sRestClientConfig)
	if err != nil {
		return fmt.Errorf("failed to build extensions client: %s", err)
	}

	plugin.watchPolicies()
	plugin.watchPods()
	plugin.watchNamespaces()

	return nil
}

func (plugin *Plugin) watchPolicies() {
	listWatch := cache.NewListWatchFromClient(plugin.k8sRestClient, "networkpolicies", "", fields.Everything())
	plugin.k8sPolicyStore, plugin.k8sPolicyController = cache.NewInformer(
		listWatch,
		&clientapi_v1beta1.NetworkPolicy{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				jsn, _ := json.Marshal(obj)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "policy": jsn}).
					Info("Network policy added")
			},
			DeleteFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				jsn, _ := json.Marshal(obj)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "policy": jsn}).
					Info("Network policy deleted")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldKey, _ := cache.MetaNamespaceKeyFunc(oldObj)
				oldJsn, _ := json.Marshal(oldObj)
				newKey, _ := cache.MetaNamespaceKeyFunc(newObj)
				newJsn, _ := json.Marshal(newObj)
				plugin.Log.WithFields(map[string]interface{}{"old-key": oldKey, "old-policy": oldJsn,
					"new-key": newKey, "new-policy": newJsn}).Info("Network policy changed")
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
	listWatch := cache.NewListWatchFromClient(plugin.k8sRestClient, "pods", "", fields.Everything())
	plugin.k8sPodStore, plugin.k8sPodController = cache.NewInformer(
		listWatch,
		&clientapi_v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				jsn, _ := json.Marshal(obj)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "pod": jsn}).
					Info("Pod added")
			},
			DeleteFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				jsn, _ := json.Marshal(obj)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "pod": jsn}).
					Info("Pod deleted")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldKey, _ := cache.MetaNamespaceKeyFunc(oldObj)
				oldJsn, _ := json.Marshal(oldObj)
				newKey, _ := cache.MetaNamespaceKeyFunc(newObj)
				newJsn, _ := json.Marshal(newObj)
				plugin.Log.WithFields(map[string]interface{}{"old-key": oldKey, "old-pod": oldJsn,
					"new-key": newKey, "new-pod": newJsn}).Info("Pod changed")
			},
		},
	)

	go plugin.k8sPodController.Run(plugin.stopCh)
	plugin.Log.Debug("Waiting to sync with Kubernetes API (Pod)")
	for !plugin.k8sPodController.HasSynced() {
	}
	plugin.Log.Debug("Finished syncing with Kubernetes API (Pod)")
}

func (plugin *Plugin) watchNamespaces() {
	listWatch := cache.NewListWatchFromClient(plugin.k8sRestClient, "namespace", "", fields.Everything())
	plugin.k8sNamespaceStore, plugin.k8sNamespaceController = cache.NewInformer(
		listWatch,
		&clientapi_v1.Namespace{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				jsn, _ := json.Marshal(obj)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "ns": jsn}).
					Info("Namespace added")
			},
			DeleteFunc: func(obj interface{}) {
				key, _ := cache.MetaNamespaceKeyFunc(obj)
				jsn, _ := json.Marshal(obj)
				plugin.Log.WithFields(map[string]interface{}{"key": key, "ns": jsn}).
					Info("Namespace deleted")
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldKey, _ := cache.MetaNamespaceKeyFunc(oldObj)
				oldJsn, _ := json.Marshal(oldObj)
				newKey, _ := cache.MetaNamespaceKeyFunc(newObj)
				newJsn, _ := json.Marshal(newObj)
				plugin.Log.WithFields(map[string]interface{}{"old-key": oldKey, "old-ns": oldJsn,
					"new-key": newKey, "new-ns": newJsn}).Info("Namespace changed")
			},
		},
	)

	go plugin.k8sNamespaceController.Run(plugin.stopCh)
	plugin.Log.Debug("Waiting to sync with Kubernetes API (Namespace)")
	for !plugin.k8sNamespaceController.HasSynced() {
	}
	plugin.Log.Debug("Finished syncing with Kubernetes API (Namespace)")
}

func (plugin *Plugin) Close() error {
	safeclose.CloseAll(plugin.stopCh)
	return nil
}
