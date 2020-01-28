package action

import (
	"fmt"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type replicaDiff int64

const (
	increase replicaDiff = 1
	decrease replicaDiff = -1
)

type parentHandler struct {
	dynNamespacedClient dynamic.ResourceInterface
	obj                 *unstructured.Unstructured
	change              replicaDiff
}

func (p *parentHandler) getObject(name string) (*unstructured.Unstructured, bool, error) {
	if p.obj != nil {
		return p.obj, true, nil
	}

	obj, err := p.dynNamespacedClient.Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	p.obj = obj
	return obj, true, nil
}

func (p *parentHandler) setObject(obj *unstructured.Unstructured) {
	p.obj = obj
}

func (p *parentHandler) increaseOrCreate(srcParent *unstructured.Unstructured, podMap map[string]interface{}) (map[string]string, error) {
	// This is to ensure that the resource does not exist at target
	pName := srcParent.GetName()
	_, exists, err := p.getObject(pName)
	if err != nil {
		return nil, fmt.Errorf("Error getting parent: %s: %v", pName, err)
	}

	pKind := srcParent.GetObjectKind()

	labels, found, err := unstructured.NestedStringMap(srcParent.Object, "spec", "selector", "matchLabels")
	if err != nil || !found {
		return nil, fmt.Errorf("no matchLabels found in parent %s %s: %v", pKind, pName, err)
	}

	if !exists {
		dstParent := srcParent.DeepCopy()
		dstParent.SetResourceVersion("")
		dstParent.SetGeneration(int64(0))
		dstParent.SetCreationTimestamp(metav1.Time{})
		dstParent.SetDeletionTimestamp(nil)
		dstParent.SetDeletionGracePeriodSeconds(nil)

		if err := unstructured.SetNestedStringMap(podMap, labels, "metadata", "labels"); err != nil {
			return nil, fmt.Errorf("error setting right labels into podSpec %s %s: %v", pKind, pName, err)
		}

		if err := unstructured.SetNestedMap(dstParent.Object, podMap, "spec", "template"); err != nil {
			return nil, fmt.Errorf("error setting podspec into unstructured %s %s: %v", pKind, pName, err)
		}

		// Ensure only 1 replica in the new parent in target cluster
		if err := unstructured.SetNestedField(dstParent.Object, int64(1), "spec", "replicas"); err != nil {
			return nil, fmt.Errorf("error setting replicas into unstructured %s %s: %v", pKind, pName, err)
		}
		p.obj = dstParent
		return nil, p.createResource(pName, p.dynNamespacedClient)
	}

	_, err = p.updateReplicas(pName, increase)
	return labels, err
}

func (p *parentHandler) decreaseOrDelete(name string) error {
	newReplicas, err := p.updateReplicas(name, decrease)
	if err != nil {
		return err
	}

	if newReplicas <= 0 {
		err := p.deleteResource(name, p.dynNamespacedClient)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (p *parentHandler) createResource(name string, client dynamic.ResourceInterface) error {
	if p.obj == nil {
		return fmt.Errorf("The target object: %s is not set for creation", name)
	}
	res := p.obj

	// TODO: do a checked creation; create and check if created with a timeout
	_, err := client.Create(res, metav1.CreateOptions{})
	klog.V(3).Infof("Created parent %s %s at target.", res.GetObjectKind(), res.GetName())
	return err
}

func (p *parentHandler) deleteResource(name string, client dynamic.ResourceInterface) error {
	// TODO: do a checked delete; delete and confirm deletion with a timeout
	err := client.Delete(name, &metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (p *parentHandler) updateReplicas(pName string, diff replicaDiff) (int64, error) {
	obj, exists, err := p.getObject(pName)
	if err != nil {
		return int64(0), err
	}
	if !exists {
		return int64(0), fmt.Errorf("No parent (does not exist) to update replicas: %s", pName)
	}

	kind := obj.GetKind()
	objName := fmt.Sprintf("%s/%s", p.obj.GetNamespace(), p.obj.GetName())

	currentReplicas, ok, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil || !ok {
		return 0, fmt.Errorf("Error retrieving 'replicas' from %s: %v", objName, err)
	}

	newReplicas := currentReplicas + int64(diff)
	if newReplicas <= 0 {
		return newReplicas, nil
	}

	err = unstructured.SetNestedField(obj.Object, newReplicas, "spec", "replicas")
	if err != nil {
		return newReplicas, fmt.Errorf("error setting replicas into unstructured %s %s: %v", kind, objName, err)
	}

	_, err = p.dynNamespacedClient.Update(obj, metav1.UpdateOptions{})
	return newReplicas, err
}
