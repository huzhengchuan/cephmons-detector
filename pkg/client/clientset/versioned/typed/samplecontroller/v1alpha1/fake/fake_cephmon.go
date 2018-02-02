/*
Copyright 2018 The Kubernetes Authors.

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

package fake

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1alpha1 "k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1"
)

// FakeCephmons implements CephmonInterface
type FakeCephmons struct {
	Fake *FakeSamplecontrollerV1alpha1
	ns   string
}

var cephmonsResource = schema.GroupVersionResource{Group: "samplecontroller.k8s.io", Version: "v1alpha1", Resource: "cephmons"}

var cephmonsKind = schema.GroupVersionKind{Group: "samplecontroller.k8s.io", Version: "v1alpha1", Kind: "Cephmon"}

// Get takes name of the cephmon, and returns the corresponding cephmon object, and an error if there is any.
func (c *FakeCephmons) Get(name string, options v1.GetOptions) (result *v1alpha1.Cephmon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(cephmonsResource, c.ns, name), &v1alpha1.Cephmon{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cephmon), err
}

// List takes label and field selectors, and returns the list of Cephmons that match those selectors.
func (c *FakeCephmons) List(opts v1.ListOptions) (result *v1alpha1.CephmonList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(cephmonsResource, cephmonsKind, c.ns, opts), &v1alpha1.CephmonList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.CephmonList{}
	for _, item := range obj.(*v1alpha1.CephmonList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested cephmons.
func (c *FakeCephmons) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(cephmonsResource, c.ns, opts))

}

// Create takes the representation of a cephmon and creates it.  Returns the server's representation of the cephmon, and an error, if there is any.
func (c *FakeCephmons) Create(cephmon *v1alpha1.Cephmon) (result *v1alpha1.Cephmon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(cephmonsResource, c.ns, cephmon), &v1alpha1.Cephmon{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cephmon), err
}

// Update takes the representation of a cephmon and updates it. Returns the server's representation of the cephmon, and an error, if there is any.
func (c *FakeCephmons) Update(cephmon *v1alpha1.Cephmon) (result *v1alpha1.Cephmon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(cephmonsResource, c.ns, cephmon), &v1alpha1.Cephmon{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cephmon), err
}

// Delete takes name of the cephmon and deletes it. Returns an error if one occurs.
func (c *FakeCephmons) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(cephmonsResource, c.ns, name), &v1alpha1.Cephmon{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCephmons) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(cephmonsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.CephmonList{})
	return err
}

// Patch applies the patch and returns the patched cephmon.
func (c *FakeCephmons) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Cephmon, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(cephmonsResource, c.ns, name, data, subresources...), &v1alpha1.Cephmon{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.Cephmon), err
}
