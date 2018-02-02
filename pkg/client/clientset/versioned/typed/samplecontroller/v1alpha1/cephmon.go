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

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1"
	scheme "k8s.io/sample-controller/pkg/client/clientset/versioned/scheme"
)

// CephmonsGetter has a method to return a CephmonInterface.
// A group's client should implement this interface.
type CephmonsGetter interface {
	Cephmons(namespace string) CephmonInterface
}

// CephmonInterface has methods to work with Cephmon resources.
type CephmonInterface interface {
	Create(*v1alpha1.Cephmon) (*v1alpha1.Cephmon, error)
	Update(*v1alpha1.Cephmon) (*v1alpha1.Cephmon, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.Cephmon, error)
	List(opts v1.ListOptions) (*v1alpha1.CephmonList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Cephmon, err error)
	CephmonExpansion
}

// cephmons implements CephmonInterface
type cephmons struct {
	client rest.Interface
	ns     string
}

// newCephmons returns a Cephmons
func newCephmons(c *SamplecontrollerV1alpha1Client, namespace string) *cephmons {
	return &cephmons{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the cephmon, and returns the corresponding cephmon object, and an error if there is any.
func (c *cephmons) Get(name string, options v1.GetOptions) (result *v1alpha1.Cephmon, err error) {
	result = &v1alpha1.Cephmon{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cephmons").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Cephmons that match those selectors.
func (c *cephmons) List(opts v1.ListOptions) (result *v1alpha1.CephmonList, err error) {
	result = &v1alpha1.CephmonList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cephmons").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested cephmons.
func (c *cephmons) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("cephmons").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a cephmon and creates it.  Returns the server's representation of the cephmon, and an error, if there is any.
func (c *cephmons) Create(cephmon *v1alpha1.Cephmon) (result *v1alpha1.Cephmon, err error) {
	result = &v1alpha1.Cephmon{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("cephmons").
		Body(cephmon).
		Do().
		Into(result)
	return
}

// Update takes the representation of a cephmon and updates it. Returns the server's representation of the cephmon, and an error, if there is any.
func (c *cephmons) Update(cephmon *v1alpha1.Cephmon) (result *v1alpha1.Cephmon, err error) {
	result = &v1alpha1.Cephmon{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cephmons").
		Name(cephmon.Name).
		Body(cephmon).
		Do().
		Into(result)
	return
}

// Delete takes name of the cephmon and deletes it. Returns an error if one occurs.
func (c *cephmons) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cephmons").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *cephmons) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cephmons").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched cephmon.
func (c *cephmons) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Cephmon, err error) {
	result = &v1alpha1.Cephmon{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("cephmons").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
