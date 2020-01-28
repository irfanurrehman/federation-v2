package action

import (
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"
)

// k8sControllerUpdater defines a specific k8s controller resource
// and the mechanism to obtain and update it
type actionExecutor struct {
	srcParentHandler parentHandler
	dstParentHandler parentHandler

	// TODO: This could change to a resource collection by definition
	// when the time comes.
	srcPod *corev1.Pod

	srcClient dynamic.Interface
	dstClient dynamic.Interface

	parentResource schema.GroupVersionResource
	podResource    schema.GroupVersionResource
	parentName     string

	dstNodeName string
}

// newK8sControllerUpdater returns a k8sControllerUpdater based on the parent kind of a pod
func newActionExecutor(srcClient, dstClient dynamic.Interface, pod *corev1.Pod, dstNodeName string) (*actionExecutor, error) {
	// Find parent kind of the pod
	pKind, pName, err := GetPodGrandInfo(srcClient, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent info of pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	var res schema.GroupVersionResource
	switch pKind {
	case KindReplicationController:
		res = schema.GroupVersionResource{
			Group:    K8sAPIReplicationControllerGV.Group,
			Version:  K8sAPIReplicationControllerGV.Version,
			Resource: ReplicationControllerResName}
	case KindReplicaSet:
		res = schema.GroupVersionResource{
			Group:    K8sAPIDeploymentGV.Group,
			Version:  K8sAPIDeploymentGV.Version,
			Resource: ReplicaSetResName}
	case KindDeployment:
		res = schema.GroupVersionResource{
			Group:    K8sAPIReplicasetGV.Group,
			Version:  K8sAPIReplicasetGV.Version,
			Resource: DeploymentResName}
	case KindDeploymentConfig:
		res = schema.GroupVersionResource{
			Group:    OpenShiftAPIDeploymentConfigGV.Group,
			Version:  OpenShiftAPIDeploymentConfigGV.Version,
			Resource: DeploymentConfigResName}
	default:
		// TODO: dont miss the case that a standalone pod can exist
		err := fmt.Errorf("unsupport controller type %s for pod %s/%s", pKind, pod.Namespace, pod.Name)
		return nil, err
	}
	return &actionExecutor{
		srcParentHandler: parentHandler{
			dynNamespacedClient: srcClient.Resource(res).Namespace(pod.Namespace),
			change:              decrease,
		},
		dstParentHandler: parentHandler{
			dynNamespacedClient: dstClient.Resource(res).Namespace(pod.Namespace),
			change:              increase,
		},
		parentName:     pName,
		parentResource: res,
		srcClient:      srcClient,
		dstClient:      dstClient,
		srcPod:         pod,
		podResource: schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "pods",
		},
		dstNodeName: dstNodeName,
	}, nil
}

func (a *actionExecutor) executeAction() error {
	podKey := fmt.Sprintf("%s/%s", a.srcPod.Namespace, a.srcPod.Name)

	/* As of now we do not place the pod
	   on the exact node listed, as there is no clean way of doing that.
	   A hacky way can be implemented if needed.
	err := a.createPodAtDestination()
	if apierrors.IsAlreadyExists(err) {
		// nothing to do repeat action
		return nil
	}
	if err != nil {
		return fmt.Errorf("Could not create the pod at destination: %s: %v", podKey, err)
	}
	// Works with B below.
	*/

	srcParent, found, err := a.srcParentHandler.getObject(a.parentName)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("Missing pod parent at source, or unsupported parent: %s", podKey)
	}

	newPod := &corev1.Pod{}
	copyPodWithoutLabel(a.srcPod, newPod)
	// We do not set a name and ns in the template
	newPod.Name = ""
	newPod.GenerateName = ""

	uPod, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&newPod)
	if err != nil {
		return err
	}

	_, err = a.dstParentHandler.increaseOrCreate(srcParent, uPod)
	if err != nil {
		return err
	}

	/*
		// B.
		err = a.updatePodLabelsAtDestination(labels)
		if err != nil {
			return err
		}*/

	err = a.deletePodFromSource()
	if err != nil {
		return err
	}

	return a.srcParentHandler.decreaseOrDelete(a.parentName)
}

func (a *actionExecutor) updatePodLabelsAtDestination(labels map[string]string) error {
	// Get the newly created pod from destination, which does not so far has parent labels
	dstPod, err := a.dstClient.Resource(a.podResource).Namespace(a.srcPod.Namespace).Get(a.srcPod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not refetch src pod: %s/%s to a typed structure, %v", a.srcPod.Namespace, a.srcPod.Name, err)
	}

	dstPod.SetLabels(labels)
	// Update with labels for the parent to adopt it
	_, err = a.dstClient.Resource(a.podResource).Namespace(a.srcPod.Namespace).Update(dstPod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("Could not update labels in destination pod: %s/%s: %v", a.srcPod.Namespace, a.srcPod.Name, err)
	}

	return nil
}

func (a *actionExecutor) createPodAtDestination() error {
	srcPod := a.srcPod
	newPod := &corev1.Pod{}
	copyPodWithoutLabel(srcPod, newPod)
	newPod.Spec.NodeName = a.dstNodeName

	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&newPod)
	if err != nil {
		return err
	}

	uPod := &unstructured.Unstructured{Object: object}
	srcPodKey := fmt.Sprintf("%s/%s", srcPod.Namespace, srcPod.Name)
	// Check if the pod already exists at destination
	_, err = a.dstClient.Resource(a.podResource).Namespace(a.srcPod.Namespace).Get(a.srcPod.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = a.dstClient.Resource(a.podResource).Namespace(a.srcPod.Namespace).Create(uPod, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	klog.V(1).Infof("Created pod:%q as %v at destination", srcPodKey, uPod)
	return nil
}

func (a *actionExecutor) deletePodFromSource() error {
	klog.V(2).Infof("Deleting pod %s/%s from src", a.srcPod.Namespace, a.srcPod.Name)
	err := a.srcClient.Resource(a.podResource).Namespace(a.srcPod.Namespace).Delete(a.srcPod.Name, &metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func parseOwnerReferences(owners []metav1.OwnerReference) (string, string) {
	for i := range owners {
		owner := &owners[i]

		if owner == nil || owner.Controller == nil {
			klog.Warningf("Nil OwnerReference")
			continue
		}

		if *(owner.Controller) && len(owner.Kind) > 0 && len(owner.Name) > 0 {
			return owner.Kind, owner.Name
		}
	}

	return "", ""
}

// GetPodParentInfo gets parent information of a pod
func GetPodParentInfo(pod *corev1.Pod) (string, string, error) {
	//1. check ownerReferences:
	if pod.OwnerReferences != nil && len(pod.OwnerReferences) > 0 {
		kind, name := parseOwnerReferences(pod.OwnerReferences)
		if len(kind) > 0 && len(name) > 0 {
			return kind, name, nil
		}
	}

	klog.V(4).Infof("no parent-info for pod-%v/%v in OwnerReferences.", pod.Namespace, pod.Name)

	//2. check annotations:
	if pod.Annotations != nil && len(pod.Annotations) > 0 {
		key := "kubernetes.io/created-by"
		if value, ok := pod.Annotations[key]; ok {

			var ref corev1.SerializedReference

			if err := json.Unmarshal([]byte(value), &ref); err != nil {
				err = fmt.Errorf("failed to decode parent annoation: %v", err)
				klog.Errorf("%v\n%v", err, value)
				return "", "", err
			}

			return ref.Reference.Kind, ref.Reference.Name, nil
		}
	}

	klog.V(4).Infof("no parent-info for pod-%v/%v in Annotations.", pod.Namespace, pod.Name)

	return "", "", nil
}

// GetPodGrandInfo gets grandParent (parent's parent) information of a pod: kind, name
// If parent does not have parent, then return parent info.
// Note: if parent kind is "ReplicaSet", then its parent's parent can be a "Deployment"
//       or if its a "ReplicationController" its parent could be "DeploymentConfig" (as in openshift).
func GetPodGrandInfo(dynClient dynamic.Interface, pod *corev1.Pod) (string, string, error) {
	//1. get Parent info: kind and name;
	kind, name, err := GetPodParentInfo(pod)
	if err != nil {
		return "", "", err
	}

	//2. if parent is "ReplicaSet" or "ReplicationController", check parent's parent
	var res schema.GroupVersionResource
	switch kind {
	case KindReplicationController:
		res = schema.GroupVersionResource{
			Group:    K8sAPIReplicationControllerGV.Group,
			Version:  K8sAPIReplicationControllerGV.Version,
			Resource: ReplicationControllerResName}
	case KindReplicaSet:
		res = schema.GroupVersionResource{
			Group:    K8sAPIDeploymentGV.Group,
			Version:  K8sAPIDeploymentGV.Version,
			Resource: ReplicaSetResName}
	default:
		return kind, name, nil
	}

	obj, err := dynClient.Resource(res).Namespace(pod.Namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		err = fmt.Errorf("Failed to get %s[%v/%v]: %v", kind, pod.Namespace, name, err)
		klog.Error(err.Error())
		return "", "", err
	}
	//2.2 get parent's parent info by parsing ownerReferences:
	rsOwnerReferences := obj.GetOwnerReferences()
	if rsOwnerReferences != nil && len(rsOwnerReferences) > 0 {
		gkind, gname := parseOwnerReferences(rsOwnerReferences)
		if len(gkind) > 0 && len(gname) > 0 {
			return gkind, gname, nil
		}
	}

	return kind, name, nil
}
