/***
Copyright 2019 Cisco Systems Inc. All rights reserved.

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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"time"

	v1 "github.com/noironetworks/aci-containers/pkg/snatpolicy/apis/aci.snat/v1"
	scheme "github.com/noironetworks/aci-containers/pkg/snatpolicy/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SnatPoliciesGetter has a method to return a SnatPolicyInterface.
// A group's client should implement this interface.
type SnatPoliciesGetter interface {
	SnatPolicies(namespace string) SnatPolicyInterface
}

// SnatPolicyInterface has methods to work with SnatPolicy resources.
type SnatPolicyInterface interface {
	Create(*v1.SnatPolicy) (*v1.SnatPolicy, error)
	Update(*v1.SnatPolicy) (*v1.SnatPolicy, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.SnatPolicy, error)
	List(opts metav1.ListOptions) (*v1.SnatPolicyList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SnatPolicy, err error)
	SnatPolicyExpansion
}

// snatPolicies implements SnatPolicyInterface
type snatPolicies struct {
	client rest.Interface
	ns     string
}

// newSnatPolicies returns a SnatPolicies
func newSnatPolicies(c *AciV1Client, namespace string) *snatPolicies {
	return &snatPolicies{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the snatPolicy, and returns the corresponding snatPolicy object, and an error if there is any.
func (c *snatPolicies) Get(name string, options metav1.GetOptions) (result *v1.SnatPolicy, err error) {
	result = &v1.SnatPolicy{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("snatpolicies").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SnatPolicies that match those selectors.
func (c *snatPolicies) List(opts metav1.ListOptions) (result *v1.SnatPolicyList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.SnatPolicyList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("snatpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested snatPolicies.
func (c *snatPolicies) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("snatpolicies").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a snatPolicy and creates it.  Returns the server's representation of the snatPolicy, and an error, if there is any.
func (c *snatPolicies) Create(snatPolicy *v1.SnatPolicy) (result *v1.SnatPolicy, err error) {
	result = &v1.SnatPolicy{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("snatpolicies").
		Body(snatPolicy).
		Do().
		Into(result)
	return
}

// Update takes the representation of a snatPolicy and updates it. Returns the server's representation of the snatPolicy, and an error, if there is any.
func (c *snatPolicies) Update(snatPolicy *v1.SnatPolicy) (result *v1.SnatPolicy, err error) {
	result = &v1.SnatPolicy{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("snatpolicies").
		Name(snatPolicy.Name).
		Body(snatPolicy).
		Do().
		Into(result)
	return
}

// Delete takes name of the snatPolicy and deletes it. Returns an error if one occurs.
func (c *snatPolicies) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("snatpolicies").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *snatPolicies) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("snatpolicies").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched snatPolicy.
func (c *snatPolicies) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SnatPolicy, err error) {
	result = &v1.SnatPolicy{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("snatpolicies").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
