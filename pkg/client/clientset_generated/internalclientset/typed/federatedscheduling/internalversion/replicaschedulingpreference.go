/*
Copyright 2018 The Federation v2 Authors.

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
package internalversion

import (
	federatedscheduling "github.com/kubernetes-sigs/federation-v2/pkg/apis/federatedscheduling"
	scheme "github.com/kubernetes-sigs/federation-v2/pkg/client/clientset_generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// ReplicaSchedulingPreferencesGetter has a method to return a ReplicaSchedulingPreferenceInterface.
// A group's client should implement this interface.
type ReplicaSchedulingPreferencesGetter interface {
	ReplicaSchedulingPreferences(namespace string) ReplicaSchedulingPreferenceInterface
}

// ReplicaSchedulingPreferenceInterface has methods to work with ReplicaSchedulingPreference resources.
type ReplicaSchedulingPreferenceInterface interface {
	Create(*federatedscheduling.ReplicaSchedulingPreference) (*federatedscheduling.ReplicaSchedulingPreference, error)
	Update(*federatedscheduling.ReplicaSchedulingPreference) (*federatedscheduling.ReplicaSchedulingPreference, error)
	UpdateStatus(*federatedscheduling.ReplicaSchedulingPreference) (*federatedscheduling.ReplicaSchedulingPreference, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*federatedscheduling.ReplicaSchedulingPreference, error)
	List(opts v1.ListOptions) (*federatedscheduling.ReplicaSchedulingPreferenceList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *federatedscheduling.ReplicaSchedulingPreference, err error)
	ReplicaSchedulingPreferenceExpansion
}

// replicaSchedulingPreferences implements ReplicaSchedulingPreferenceInterface
type replicaSchedulingPreferences struct {
	client rest.Interface
	ns     string
}

// newReplicaSchedulingPreferences returns a ReplicaSchedulingPreferences
func newReplicaSchedulingPreferences(c *FederatedschedulingClient, namespace string) *replicaSchedulingPreferences {
	return &replicaSchedulingPreferences{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the replicaSchedulingPreference, and returns the corresponding replicaSchedulingPreference object, and an error if there is any.
func (c *replicaSchedulingPreferences) Get(name string, options v1.GetOptions) (result *federatedscheduling.ReplicaSchedulingPreference, err error) {
	result = &federatedscheduling.ReplicaSchedulingPreference{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ReplicaSchedulingPreferences that match those selectors.
func (c *replicaSchedulingPreferences) List(opts v1.ListOptions) (result *federatedscheduling.ReplicaSchedulingPreferenceList, err error) {
	result = &federatedscheduling.ReplicaSchedulingPreferenceList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested replicaSchedulingPreferences.
func (c *replicaSchedulingPreferences) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a replicaSchedulingPreference and creates it.  Returns the server's representation of the replicaSchedulingPreference, and an error, if there is any.
func (c *replicaSchedulingPreferences) Create(replicaSchedulingPreference *federatedscheduling.ReplicaSchedulingPreference) (result *federatedscheduling.ReplicaSchedulingPreference, err error) {
	result = &federatedscheduling.ReplicaSchedulingPreference{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		Body(replicaSchedulingPreference).
		Do().
		Into(result)
	return
}

// Update takes the representation of a replicaSchedulingPreference and updates it. Returns the server's representation of the replicaSchedulingPreference, and an error, if there is any.
func (c *replicaSchedulingPreferences) Update(replicaSchedulingPreference *federatedscheduling.ReplicaSchedulingPreference) (result *federatedscheduling.ReplicaSchedulingPreference, err error) {
	result = &federatedscheduling.ReplicaSchedulingPreference{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		Name(replicaSchedulingPreference.Name).
		Body(replicaSchedulingPreference).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *replicaSchedulingPreferences) UpdateStatus(replicaSchedulingPreference *federatedscheduling.ReplicaSchedulingPreference) (result *federatedscheduling.ReplicaSchedulingPreference, err error) {
	result = &federatedscheduling.ReplicaSchedulingPreference{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		Name(replicaSchedulingPreference.Name).
		SubResource("status").
		Body(replicaSchedulingPreference).
		Do().
		Into(result)
	return
}

// Delete takes name of the replicaSchedulingPreference and deletes it. Returns an error if one occurs.
func (c *replicaSchedulingPreferences) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *replicaSchedulingPreferences) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched replicaSchedulingPreference.
func (c *replicaSchedulingPreferences) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *federatedscheduling.ReplicaSchedulingPreference, err error) {
	result = &federatedscheduling.ReplicaSchedulingPreference{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("replicaschedulingpreferences").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
