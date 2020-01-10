/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ActionType string
type Condition string

const (
	ActionType_NONE ActionType = "none"
	ActionType_MOVE ActionType = "move"

	Condition_UNKNOWN   Condition = "unknown"
	Condition_EXECUTING Condition = "executing"
	Condition_COMPLETE  Condition = "complete"
	Condition_FAILED    Condition = "failed"
)

// ObjectReference contains enough information to let you identify the referred target resource.
type ObjectReference struct {
	// Kind of the target; More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds"
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Name of the target; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Namespace of the target.
	Namespace string `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`
	// API version of the target.
	// +optional
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,4,opt,name=apiVersion"`
}

// Clusters contains the names of source and destination clusters for the Action.
type Clusters struct {
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
}

// ActionSpec defines the desired state of Action
type ActionSpec struct {
	// Selector is a label query over kinds that this Action targets.
	// This is to be used when multiple resources are part of this Action.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// TODO: useful future reference
	// Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// ComponentGroupKinds is a list of Kinds for Action's target components (e.g. Deployments, Pods, Services, CRDs).
	// It should ideally be used in conjunction with the Actions's Selector to perform actions on the target components.
	// TODO: useful future reference
	// ComponentGroupKinds []metav1.GroupKind `json:"componentKinds,omitempty"`

	// TargetRef can be used instead of selector and ComponentGroupKinds to identify a single specific resource.
	// TODO: TargetRef will be used and selector discarded if both selector and TargetRef are specified.
	TargetRef ObjectReference `json:"targetRef"`

	// TargetNode is an optional name of the node in target cluster to be used for the move action.
	TargetNode string `json:"targetNode,omitempty"`

	// Clusters specifies the source and destination cluster names as registered with kubefed.
	Clusters Clusters `json:"clusters"`

	// ActionType specifies the type of action to be performed on the target resource.
	// Currently only supported type will be "move". An empty value will be considered as "move".
	// TODO: Use these values from turbo sdk if need be
	ActionType ActionType `json:"actionType,omitempty"`
}

// ActionStatus defines the observed state of Action
type ActionStatus struct {
	// Condition represents the present state the Action is currently in.
	Condition Condition `json:"condition,omitempty"`

	// Represents time when the Action was acknowledged by the Action controller.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Represents time when the Action was completed or it determined failed by the Action controller.
	// It is represented in RFC3339 form and is in UTC.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Message optionally provides a human readable message especially for failure.
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Action is the Schema for the actions API
// +k8s:openapi-gen=true
type Action struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ActionSpec   `json:"spec,omitempty"`
	Status ActionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ActionList contains a list of Action
type ActionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Action `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Action{}, &ActionList{})
}
