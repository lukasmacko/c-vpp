package reflector

import (
	"sync"

	clientapi_metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientapi_v1 "k8s.io/client-go/pkg/api/v1"
	clientapi_v1beta1 "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"

	proto "github.com/contiv/contiv-vpp/plugins/reflector/model/policy"
)

type PolicyReflector struct {
	ReflectorDeps

	stopCh <-chan struct{}
	wg     *sync.WaitGroup

	k8sPolicyStore      cache.Store
	k8sPolicyController cache.Controller
}

func (pr *PolicyReflector) Init(stopCh2 <-chan struct{}, wg *sync.WaitGroup) error {
	pr.stopCh = stopCh2
	pr.wg = wg

	restClient := pr.K8sClientset.ExtensionsV1beta1().RESTClient()
	listWatch := cache.NewListWatchFromClient(restClient, "networkpolicies", "", fields.Everything())
	pr.k8sPolicyStore, pr.k8sPolicyController = cache.NewInformer(
		listWatch,
		&clientapi_v1beta1.NetworkPolicy{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				policy, ok := obj.(*clientapi_v1beta1.NetworkPolicy)
				if !ok {
					pr.Log.Warn("Failed to cast newly created policy object")
				} else {
					pr.AddPolicy(policy)
				}
			},
			DeleteFunc: func(obj interface{}) {
				policy, ok := obj.(*clientapi_v1beta1.NetworkPolicy)
				if !ok {
					pr.Log.Warn("Failed to cast removed policy object")
				} else {
					pr.DeletePolicy(policy)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				policyOld, ok1 := oldObj.(*clientapi_v1beta1.NetworkPolicy)
				policyNew, ok2 := newObj.(*clientapi_v1beta1.NetworkPolicy)
				if !ok1 || !ok2 {
					pr.Log.Warn("Failed to cast changed policy object")
				} else {
					pr.UpdatePolicy(policyNew, policyOld)
				}
			},
		},
	)

	pr.wg.Add(1)
	go pr.Run()

	return nil
}

func (pr *PolicyReflector) AddPolicy(policy *clientapi_v1beta1.NetworkPolicy) {
	pr.Log.WithField("policy", policy).Info("Policy added")
	policyProto := pr.PolicyToProto(policy)
	key := proto.PolicyKey(policy.GetName(), policy.GetNamespace())
	err := pr.Publish.Put(key, policyProto)
	if err != nil {
		pr.Log.WithField("err", err).Warn("Failed to add policy state data into the data store")
	}
}

func (pr *PolicyReflector) DeletePolicy(policy *clientapi_v1beta1.NetworkPolicy) {
	pr.Log.WithField("policy", policy).Info("Policy removed")
	// TODO (Delete not yet supported by kvdbsync)
	//key := proto.PolicyKey(policy.GetName(), policy.GetNamespace())
	//err := pr.Publish.Delete(key)
	//if err != nil {
	//	pr.Log.WithField("err", err).Warn("Failed to remove policy state data from the data store")
	//}
}

func (pr *PolicyReflector) UpdatePolicy(policyNew, policyOld *clientapi_v1beta1.NetworkPolicy) {
	pr.Log.WithFields(map[string]interface{}{"policy-old": policyOld, "policy-new": policyNew}).Info("Policy updated")
	policyProto := pr.PolicyToProto(policyNew)
	key := proto.PolicyKey(policyNew.GetName(), policyNew.GetNamespace())
	err := pr.Publish.Put(key, policyProto)
	if err != nil {
		pr.Log.WithField("err", err).Warn("Failed to update policy state data in the data store")
	}
}

func (pr *PolicyReflector) PolicyToProto(policy *clientapi_v1beta1.NetworkPolicy) *proto.Policy {
	policyProto := &proto.Policy{}
	// Name
	policyProto.Name = policy.GetName()
	policyProto.Namespace = policy.GetNamespace()
	// Labels
	labels := policy.GetLabels()
	if labels != nil {
		for key, val := range labels {
			policyProto.Label = append(policyProto.Label, &proto.Policy_Label{Key: key, Value: val})
		}
	}
	// Pods
	policyProto.Pods = pr.LabelSelectorToProto(&policy.Spec.PodSelector)
	// Ingress rules
	if policy.Spec.Ingress != nil {
		for _, ingress := range policy.Spec.Ingress {
			ingressProto := &proto.Policy_IngressRule{}
			// Ports
			if ingress.Ports != nil {
				for _, port := range ingress.Ports {
					portProto := &proto.Policy_IngressRule_Port{}
					// Protocol
					if port.Protocol != nil {
						switch *port.Protocol {
						case clientapi_v1.ProtocolTCP:
							portProto.Protocol = proto.Policy_IngressRule_Port_TCP
						case clientapi_v1.ProtocolUDP:
							portProto.Protocol = proto.Policy_IngressRule_Port_UDP
						}
					}
					// Port number/name
					if port.Port != nil {
						switch port.Port.Type {
						case intstr.Int:
							portProto.Port.Type = proto.Policy_IngressRule_Port_PortNameOrNumber_NUMBER
							portProto.Port.Number = port.Port.IntVal
						case intstr.String:
							portProto.Port.Type = proto.Policy_IngressRule_Port_PortNameOrNumber_NUMBER
							portProto.Port.Number = port.Port.IntVal
						}
					}
					// append port
					ingressProto.Port = append(ingressProto.Port, portProto)
				}
			}
			// From
			if ingress.From != nil {
				for _, from := range ingress.From {
					fromProto := &proto.Policy_IngressRule_Peer{}
					// pod selectors
					if from.PodSelector != nil {
						fromProto.Pods = pr.LabelSelectorToProto(from.PodSelector)
					} else if from.NamespaceSelector != nil {
						// namespace selectors
						fromProto.Namespaces = pr.LabelSelectorToProto(from.NamespaceSelector)
					}
					// append peer
					ingressProto.From = append(ingressProto.From, fromProto)
				}
			}
			// append rule
			policyProto.IngressRule = append(policyProto.IngressRule, ingressProto)
		}
	}
	return policyProto
}

func (pr *PolicyReflector) LabelSelectorToProto(selector *clientapi_metav1.LabelSelector) *proto.Policy_LabelSelector {
	selectorProto := &proto.Policy_LabelSelector{}
	// MatchLabels
	if selector.MatchLabels != nil {
		for key, val := range selector.MatchLabels {
			selectorProto.MatchLabel = append(selectorProto.MatchLabel, &proto.Policy_Label{Key: key, Value: val})
		}
	}
	// MatchExpressions
	if selector.MatchExpressions != nil {
		for _, expression := range selector.MatchExpressions {
			expressionProto := &proto.Policy_LabelSelector_LabelExpression{}
			// Key
			expressionProto.Key = expression.Key
			// Operator
			switch expression.Operator {
			case clientapi_metav1.LabelSelectorOpIn:
				expressionProto.Operator = proto.Policy_LabelSelector_LabelExpression_IN
			case clientapi_metav1.LabelSelectorOpNotIn:
				expressionProto.Operator = proto.Policy_LabelSelector_LabelExpression_NOT_IN
			case clientapi_metav1.LabelSelectorOpExists:
				expressionProto.Operator = proto.Policy_LabelSelector_LabelExpression_EXISTS
			case clientapi_metav1.LabelSelectorOpDoesNotExist:
				expressionProto.Operator = proto.Policy_LabelSelector_LabelExpression_DOES_NOT_EXIST

			}
			// Values
			if expression.Values != nil {
				for _, val := range expression.Values {
					expressionProto.Value = append(expressionProto.Value, val)
				}
			}
			// append expression
			selectorProto.MatchExpression = append(selectorProto.MatchExpression, expressionProto)
		}
	}
	return selectorProto
}

func (pr *PolicyReflector) Run() {
	defer pr.wg.Done()

	pr.Log.Info("Policy reflector is now running")
	pr.k8sPolicyController.Run(pr.stopCh)
	pr.Log.Info("Stopping Policy reflector")
}

func (pr *PolicyReflector) Close() error {
	return nil
}
