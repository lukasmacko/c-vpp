package reflector

import (
	"sync"

	"k8s.io/apimachinery/pkg/fields"
	clientapi_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
)

type NamespaceReflector struct {
	ReflectorDeps

	stopCh <-chan struct{}
	wg     *sync.WaitGroup

	k8sNamespaceStore      cache.Store
	k8sNamespaceController cache.Controller
}

func (nr *NamespaceReflector) Init(stopCh2 <-chan struct{}, wg *sync.WaitGroup) error {
	nr.stopCh = stopCh2
	nr.wg = wg

	restClient := nr.K8sClientset.CoreV1().RESTClient()
	listWatch := cache.NewListWatchFromClient(restClient, "namespaces", "", fields.Everything())
	nr.k8sNamespaceStore, nr.k8sNamespaceController = cache.NewInformer(
		listWatch,
		&clientapi_v1.Namespace{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ns, ok := obj.(*clientapi_v1.Namespace)
				if !ok {
					nr.Log.Warn("Failed to cast newly created namespace object")
				} else {
					nr.AddNamespace(ns)
				}
			},
			DeleteFunc: func(obj interface{}) {
				ns, ok := obj.(*clientapi_v1.Namespace)
				if !ok {
					nr.Log.Warn("Failed to cast removed namespace object")
				} else {
					nr.DeleteNamespace(ns)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				nsOld, ok1 := oldObj.(*clientapi_v1.Namespace)
				nsNew, ok2 := newObj.(*clientapi_v1.Namespace)
				if !ok1 || !ok2 {
					nr.Log.Warn("Failed to cast changed namespace object")
				} else {
					nr.UpdateNamespace(nsNew, nsOld)
				}
			},
		},
	)

	nr.wg.Add(1)
	go nr.Run()

	return nil
}

func (nr *NamespaceReflector) AddNamespace(ns *clientapi_v1.Namespace) {
	nr.Log.WithField("ns", ns).Info("Namespace added")
}

func (nr *NamespaceReflector) DeleteNamespace(ns *clientapi_v1.Namespace) {
	nr.Log.WithField("ns", ns).Info("Namespace removed")
}

func (nr *NamespaceReflector) UpdateNamespace(nsNew, nsOld *clientapi_v1.Namespace) {
	nr.Log.WithFields(map[string]interface{}{"ns-old": nsOld, "ns-new": nsNew}).Info("Namespace changed")
}

func (nr *NamespaceReflector) Run() {
	defer nr.wg.Done()

	nr.Log.Info("Namespace reflector is now running")
	nr.k8sNamespaceController.Run(nr.stopCh)
	nr.Log.Info("Stopping Namespace reflector")
}

func (nr *NamespaceReflector) Close() error {
	return nil
}
