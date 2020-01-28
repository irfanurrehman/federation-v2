package action

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"strconv"
	"strings"
	"time"
)

/*
const (
	TurboActionAnnotationKey   string = "kubeturbo.io/action"
	TurboMoveAnnotationValue   string = "move"
)

type podHandler struct {
	pod *corev1.Pod

	srcClient dynamic.Interface
	dstClient dynamic.Interface

	parentResource schema.GroupVersionResource
	parentName string

	dstNodeName string
}


*/

const (
	TurboActionAnnotationKey string = "kubeturbo.io/action"
	TurboMoveAnnotationValue string = "move"
)

func copyPodInfo(oldPod, newPod *corev1.Pod) {
	//1. typeMeta
	newPod.TypeMeta = oldPod.TypeMeta

	//2. objectMeta
	newPod.ObjectMeta = oldPod.ObjectMeta
	newPod.SelfLink = ""
	newPod.ResourceVersion = ""
	newPod.Generation = 0
	newPod.CreationTimestamp = metav1.Time{}
	newPod.DeletionTimestamp = nil
	newPod.DeletionGracePeriodSeconds = nil

	//3. podSpec
	spec := oldPod.Spec
	spec.Hostname = ""
	spec.Subdomain = ""
	spec.NodeName = ""

	//4a. Remove default volumes
	// TODO: revisit; this code assumes that there will be only one default volumemount
	containers := spec.Containers
	for i, container := range containers {
		for j, volumeMount := range container.VolumeMounts {
			if strings.Contains(volumeMount.Name, "default-token-") {
				containers[i].VolumeMounts = append(containers[i].VolumeMounts[:j], containers[i].VolumeMounts[j+1:]...)
				break
			}
		}
	}
	spec.Containers = containers

	//4b.
	for i, vol := range spec.Volumes {
		if strings.Contains(vol.Name, "default-token-") {
			spec.Volumes = append(spec.Volumes[:i], spec.Volumes[i+1:]...)
			break
		}
	}

	newPod.Spec = spec
	return
}

func copyPodWithoutLabel(oldPod, newPod *corev1.Pod) {
	copyPodInfo(oldPod, newPod)

	// set Labels and OwnerReference to be empty
	newPod.Labels = make(map[string]string)
	newPod.OwnerReferences = []metav1.OwnerReference{}
}

// Generates a name for the new pod from the old one. The new pod name will
// be the original pod name followed by "-" + current timestamp.
func genNewPodName(oldPod *corev1.Pod) string {
	oldPodName := oldPod.Name
	oriPodName := oldPodName

	// If the pod was created from Turbo actions (resize/move), the oldPodName
	// will include its timestamp. In such case, we want to find the original
	// pod name.
	if _, ok := oldPod.Annotations[TurboActionAnnotationKey]; ok {
		if idx := strings.LastIndex(oldPodName, "-"); idx >= 0 {
			oriPodName = oldPodName[:idx]
		}
	}

	// Append the pod name with current timestamp
	newPodName := oriPodName + "-" + strconv.FormatInt(time.Now().UnixNano(), 32)
	klog.V(4).Infof("Generated new pod name %s for pod %s (original pod %s)", newPodName, oldPodName, oriPodName)

	return newPodName
}
