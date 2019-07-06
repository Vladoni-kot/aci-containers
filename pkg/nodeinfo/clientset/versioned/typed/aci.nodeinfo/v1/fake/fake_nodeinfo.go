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

package fake

import (
	acinodeinfov1 "github.com/noironetworks/aci-containers/pkg/nodeinfo/apis/aci.nodeinfo/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNodeinfos implements NodeinfoInterface
type FakeNodeinfos struct {
	Fake *FakeAciV1
	ns   string
}

var nodeinfosResource = schema.GroupVersionResource{Group: "aci.nodeinfo", Version: "v1", Resource: "nodeinfos"}

var nodeinfosKind = schema.GroupVersionKind{Group: "aci.nodeinfo", Version: "v1", Kind: "Nodeinfo"}

// Get takes name of the nodeinfo, and returns the corresponding nodeinfo object, and an error if there is any.
func (c *FakeNodeinfos) Get(name string, options v1.GetOptions) (result *acinodeinfov1.Nodeinfo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(nodeinfosResource, c.ns, name), &acinodeinfov1.Nodeinfo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*acinodeinfov1.Nodeinfo), err
}

// List takes label and field selectors, and returns the list of Nodeinfos that match those selectors.
func (c *FakeNodeinfos) List(opts v1.ListOptions) (result *acinodeinfov1.NodeinfoList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(nodeinfosResource, nodeinfosKind, c.ns, opts), &acinodeinfov1.NodeinfoList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &acinodeinfov1.NodeinfoList{ListMeta: obj.(*acinodeinfov1.NodeinfoList).ListMeta}
	for _, item := range obj.(*acinodeinfov1.NodeinfoList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested nodeinfos.
func (c *FakeNodeinfos) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(nodeinfosResource, c.ns, opts))

}

// Create takes the representation of a nodeinfo and creates it.  Returns the server's representation of the nodeinfo, and an error, if there is any.
func (c *FakeNodeinfos) Create(nodeinfo *acinodeinfov1.Nodeinfo) (result *acinodeinfov1.Nodeinfo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(nodeinfosResource, c.ns, nodeinfo), &acinodeinfov1.Nodeinfo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*acinodeinfov1.Nodeinfo), err
}

// Update takes the representation of a nodeinfo and updates it. Returns the server's representation of the nodeinfo, and an error, if there is any.
func (c *FakeNodeinfos) Update(nodeinfo *acinodeinfov1.Nodeinfo) (result *acinodeinfov1.Nodeinfo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(nodeinfosResource, c.ns, nodeinfo), &acinodeinfov1.Nodeinfo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*acinodeinfov1.Nodeinfo), err
}

// Delete takes name of the nodeinfo and deletes it. Returns an error if one occurs.
func (c *FakeNodeinfos) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(nodeinfosResource, c.ns, name), &acinodeinfov1.Nodeinfo{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNodeinfos) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(nodeinfosResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &acinodeinfov1.NodeinfoList{})
	return err
}

// Patch applies the patch and returns the patched nodeinfo.
func (c *FakeNodeinfos) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *acinodeinfov1.Nodeinfo, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(nodeinfosResource, c.ns, name, pt, data, subresources...), &acinodeinfov1.Nodeinfo{})

	if obj == nil {
		return nil, err
	}
	return obj.(*acinodeinfov1.Nodeinfo), err
}
