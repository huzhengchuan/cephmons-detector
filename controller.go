/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"fmt"
	"time"
	"strconv"

	"github.com/golang/glog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/apimachinery/pkg/util/sets"

	samplev1alpha1 "k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1"
	clientset "k8s.io/sample-controller/pkg/client/clientset/versioned"
	samplescheme "k8s.io/sample-controller/pkg/client/clientset/versioned/scheme"
	informers "k8s.io/sample-controller/pkg/client/informers/externalversions"
	listers "k8s.io/sample-controller/pkg/client/listers/samplecontroller/v1alpha1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1alpha1 "k8s.io/sample-controller/pkg/apis/samplecontroller/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	podutil "k8s.io/sample-controller/pkg/pod"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	endpointsUtil "k8s.io/sample-controller/pkg/endpoints"
	"k8s.io/api/core/v1"
)

const (
	controllerAgentName = "cephmons detector controller"
	TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"
	ESHostNetWorkEndpointsAnnotation = "service.alpha.kubernetes.io/es-hostnetwork-endpoints"
)


const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)

// Controller is the controller implementation for Foo resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	podsLister        corelisters.PodLister
	podsSynced 	  cache.InformerSynced
	endpointsLister   corelisters.EndpointsLister
	endpointsSynced	  cache.InformerSynced
	servicesLister    corelisters.ServiceLister
	servicesSynced	  cache.InformerSynced
	cephmonsLister        listers.CephmonLister
	cephmonsSynced        cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	cephmonWorkqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	sampleInformerFactory informers.SharedInformerFactory) *Controller {

	// obtain references to shared index informers for the Deployment and Foo
	// types.
	podInformer := kubeInformerFactory.Core().V1().Pods()
	serviceInformer := kubeInformerFactory.Core().V1().Services()
	endpointInformer := kubeInformerFactory.Core().V1().Endpoints()
	cephmonInformer := sampleInformerFactory.Samplecontroller().V1alpha1().Cephmons()

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	samplescheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		sampleclientset:   sampleclientset,
		cephmonsLister:    cephmonInformer.Lister(),
		cephmonsSynced:    cephmonInformer.Informer().HasSynced,
		podsLister:	   podInformer.Lister(),
		podsSynced:        podInformer.Informer().HasSynced,
		servicesLister:    serviceInformer.Lister(),
		servicesSynced:    serviceInformer.Informer().HasSynced,
		endpointsLister:   endpointInformer.Lister(),
		endpointsSynced:   endpointInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Services"),
		cephmonWorkqueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Cephmons"),
		recorder:          recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Cephmon resources change
	cephmonInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.addCephmon,
		UpdateFunc: controller.updateCephmon,
		DeleteFunc: controller.deleteCephmon,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueService,
		UpdateFunc: func(old, cur interface{}) {
			controller.enqueueService(cur)
		},
		DeleteFunc: controller.enqueueService,
	})

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.deletePod,
	})


	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Cephmon controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.servicesSynced, c.cephmonsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced, c.podsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	if ok := cache.WaitForCacheSync(stopCh, c.cephmonsSynced, c.cephmonsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Cephmon resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runCephmonsWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) runCephmonsWorker() {
	for c.processNextCephmonsWorkItem() {
	}
}

func (c *Controller) processNextCephmonsWorkItem() bool {
	obj, shutdown := c.cephmonWorkqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.cephmonWorkqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.cephmonWorkqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Cephmon resource to be synced.
		if err := c.syncCephmonsHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.cephmonWorkqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Cephmon resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func toEndpointAddress(pod *corev1.Pod, cephmon *v1alpha1.Cephmon) *corev1.EndpointAddress {
	return &corev1.EndpointAddress{
		IP:       cephmon.Spec.PublicIP,
		NodeName: &pod.Spec.NodeName,
		TargetRef: &corev1.ObjectReference{
			Kind:            "Pod",
			Namespace:       pod.ObjectMeta.Namespace,
			Name:            pod.ObjectMeta.Name,
			UID:             pod.ObjectMeta.UID,
			ResourceVersion: pod.ObjectMeta.ResourceVersion,
		}}
}


func addEndpointSubset(subsets []corev1.EndpointSubset, pod *corev1.Pod, epa corev1.EndpointAddress,
	epp corev1.EndpointPort, tolerateUnreadyEndpoints bool) ([]corev1.EndpointSubset, int, int) {
	var readyEps int = 0
	var notReadyEps int = 0
	if tolerateUnreadyEndpoints || podutil.IsPodReady(pod) {
		subsets = append(subsets, corev1.EndpointSubset{
			Addresses: []corev1.EndpointAddress{epa},
			Ports:     []corev1.EndpointPort{epp},
		})
		readyEps++
	} else if shouldPodBeInEndpoints(pod) {
		glog.V(5).Infof("Pod is out of service: %v/%v", pod.Namespace, pod.Name)
		subsets = append(subsets, corev1.EndpointSubset{
			NotReadyAddresses: []corev1.EndpointAddress{epa},
			Ports:             []corev1.EndpointPort{epp},
		})
		notReadyEps++
	}
	return subsets, readyEps, notReadyEps
}

func shouldPodBeInEndpoints(pod *corev1.Pod) bool {
	switch pod.Spec.RestartPolicy {
	case corev1.RestartPolicyNever:
		return pod.Status.Phase != corev1.PodFailed && pod.Status.Phase != corev1.PodSucceeded
	case corev1.RestartPolicyOnFailure:
		return pod.Status.Phase != corev1.PodSucceeded
	default:
		return true
	}
}

func (c *Controller) syncCephmonsHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	cephmon, err := c.cephmonsLister.Cephmons(namespace).Get(name)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	podName, ok := cephmon.Labels["podName"]
	if !ok {
		c.sampleclientset.SamplecontrollerV1alpha1().Cephmons(cephmon.Namespace).Delete(cephmon.Name, nil)
		return nil
	}
	_, err = c.podsLister.Pods(cephmon.Namespace).Get(podName)
	if err != nil {
		if errors.IsNotFound(err) {
			c.sampleclientset.SamplecontrollerV1alpha1().Cephmons(cephmon.Namespace).Delete(cephmon.Name, nil)
		}
	}
	return nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Cephmon resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	service, err := c.servicesLister.Services(namespace).Get(name)
	if err != nil {
		err = c.kubeclientset.CoreV1().Endpoints(namespace).Delete(name, nil)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	esHostNetWorkAnnotation, ok := service.Annotations[ESHostNetWorkEndpointsAnnotation]
	if !ok || esHostNetWorkAnnotation != "true" {
		glog.V(5).Infof("service:%s skip, only deal to ESHostNetWorkEndpointsAnnotation service", key)
		return nil
	}

	glog.V(5).Infof("update endpoints for service:%s", key)
	cephmonLabels := make(map[string]string)
	cephmonLabels["service"] = service.Name
	cephmons, err := c.cephmonsLister.Cephmons(service.Namespace).List(labels.Set(cephmonLabels).AsSelectorPreValidated())
	if err != nil {
		return err
	}

	pods, err := c.podsLister.Pods(service.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}

	keyPods := make(map[string]*corev1.Pod)
	for _, pod := range pods {
		keyPods[pod.Name] = pod
	}

	var tolerateUnreadyEndpoints bool
	if v, ok := service.Annotations[TolerateUnreadyEndpointsAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err == nil {
			tolerateUnreadyEndpoints = b
		} else {
			utilruntime.HandleError(fmt.Errorf("Failed to parse annotation %v: %v", TolerateUnreadyEndpointsAnnotation, err))
		}
	}

	subsets := []corev1.EndpointSubset{}
	var totalReadyEps int = 0
	var totalNotReadyEps int = 0

	glog.V(4).Infof("Cephmon is %+v", cephmons)
	for _, cephmon := range cephmons {

		var podName string
		if podName, ok = cephmon.Labels["podName"]; !ok {
			c.sampleclientset.SamplecontrollerV1alpha1().Cephmons(cephmon.Namespace).Delete(cephmon.Name, nil)
			continue
		}

		pod, ok := keyPods[podName]
		if !ok {
			//delete cephmon rule  CoreV1().Endpoints(service.Namespace).Create(newEndpoints)
			c.sampleclientset.SamplecontrollerV1alpha1().Cephmons(cephmon.Namespace).Delete(cephmon.Name, nil)
			continue
		}

		if len(pod.Status.PodIP) == 0 || pod.DeletionTimestamp != nil {
			glog.V(5).Infof("Pod is being deleted %s/%s", pod.Namespace, pod.Name)
			continue
		}

		epa := *toEndpointAddress(pod, cephmon)

		hostname := pod.Spec.Hostname
		if len(hostname) > 0 && pod.Spec.Subdomain == service.Name && service.Namespace == pod.Namespace {
			epa.Hostname = hostname
		}
		// Allow headless service not to have ports.
		if len(service.Spec.Ports) == 0 {
			if service.Spec.ClusterIP == corev1.ClusterIPNone {
				epp := corev1.EndpointPort{Port: 0, Protocol: corev1.ProtocolTCP}
				subsets, totalReadyEps, totalNotReadyEps = addEndpointSubset(subsets, pod, epa, epp, tolerateUnreadyEndpoints)
			}
		} else {
			for i := range service.Spec.Ports {
				servicePort := &service.Spec.Ports[i]

				portName := servicePort.Name
				portProto := servicePort.Protocol
				portNum, err := podutil.FindPort(pod, servicePort)
				if err != nil {
					glog.V(4).Infof("Failed to find port for service %s/%s: %v", service.Namespace, service.Name, err)
					continue
				}

				var readyEps, notReadyEps int
				epp := corev1.EndpointPort{Name: portName, Port: int32(portNum), Protocol: portProto}
				subsets, readyEps, notReadyEps = addEndpointSubset(subsets, pod, epa, epp, tolerateUnreadyEndpoints)
				totalReadyEps = totalReadyEps + readyEps
				totalNotReadyEps = totalNotReadyEps + notReadyEps
			}
		}
	}
	subsets = endpointsUtil.RepackSubsets(subsets)

	// See if there's actually an update here.
	currentEndpoints, err := c.endpointsLister.Endpoints(service.Namespace).Get(service.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			currentEndpoints = &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:   service.Name,
					Labels: service.Labels,
				},
			}
		} else {
			return err
		}
	}

	createEndpoints := len(currentEndpoints.ResourceVersion) == 0

	if !createEndpoints &&
		apiequality.Semantic.DeepEqual(currentEndpoints.Subsets, subsets) &&
		apiequality.Semantic.DeepEqual(currentEndpoints.Labels, service.Labels) {
		glog.V(5).Infof("endpoints are equal for %s/%s, skipping update", service.Namespace, service.Name)
		return nil
	}
	newEndpoints := currentEndpoints.DeepCopy()
	newEndpoints.Subsets = subsets
	newEndpoints.Labels = service.Labels
	if newEndpoints.Annotations == nil {
		newEndpoints.Annotations = make(map[string]string)
	}

	glog.V(4).Infof("Update endpoints for %v/%v, ready: %d not ready: %d", service.Namespace, service.Name, totalReadyEps, totalNotReadyEps)
	if createEndpoints {
		// No previous endpoints, create them
		_, err = c.kubeclientset.CoreV1().Endpoints(service.Namespace).Create(newEndpoints)
	} else {
		// Pre-existing
		_, err = c.kubeclientset.CoreV1().Endpoints(service.Namespace).Update(newEndpoints)
	}
	if err != nil {
		if createEndpoints && errors.IsForbidden(err) {
			// A request is forbidden primarily for two reasons:
			// 1. namespace is terminating, endpoint creation is not allowed by default.
			// 2. policy is misconfigured, in which case no service would function anywhere.
			// Given the frequency of 1, we log at a lower level.
			glog.V(5).Infof("Forbidden from creating endpoints: %v", err)
		}
		return err
	}

	c.recorder.Event(service, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateCephmonStatus(cephmon *samplev1alpha1.Cephmon, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	cephmonCopy := cephmon.DeepCopy()
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Cephmon resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := c.sampleclientset.SamplecontrollerV1alpha1().Cephmons(cephmon.Namespace).Update(cephmonCopy)
	return err
}

// enqueueCephmon takes a Cephmon resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Cephmon.
func (c *Controller) enqueueCephmon(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) enqueueService(obj interface{}) {
	var key string
	var err error
	glog.V(5).Infof("enqueueService")
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) addCephmon(obj interface{}) {
	glog.V(5).Infof("addCephmon")
	cephmon := obj.(*v1alpha1.Cephmon)
	services, err := c.getCephmonServiceMemberships(cephmon)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get cephmon %v/%v's service memberships: %v", cephmon.Namespace, cephmon.Name, err))
		return
	}
	for key := range services {
		c.workqueue.AddRateLimited(key)
	}

}

func (c *Controller) deleteCephmon(obj interface{}) {
	glog.V(5).Infof("deleteCephmon")
	cephmon := obj.(*v1alpha1.Cephmon)
	services, err := c.getCephmonServiceMemberships(cephmon)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get cephmon %v/%v's service memberships: %v", cephmon.Namespace, cephmon.Name, err))
		return
	}

	for key := range services {
		c.workqueue.AddRateLimited(key)
	}
}

func (c *Controller) deletePod(obj interface{}) {
	glog.V(5).Infof("deletePod")
	pod := obj.(*v1.Pod)
	cephmons, err := c.getPodCephmonMemberships(pod)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get cephmon %v/%v's service memberships: %v", pod.Namespace, pod.Name, err))
		return
	}

	for key := range cephmons {
		c.workqueue.AddRateLimited(key)
	}
}

func (c *Controller) updateCephmon(old, cur interface{}) {
	glog.V(5).Infof("updateCephmon")
	cephmon := cur.(*v1alpha1.Cephmon)
	services, err := c.getCephmonServiceMemberships(cephmon)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to get cephmon %v/%v's service memberships: %v", cephmon.Namespace, cephmon.Name, err))
		return
	}
	for key := range services {
		c.workqueue.AddRateLimited(key)
	}

	var key string
	if key, err = cache.MetaNamespaceKeyFunc(cephmon); err != nil {
		runtime.HandleError(err)
		return
	}
	c.cephmonWorkqueue.AddRateLimited(key)

}

func (c *Controller) getPodCephmonMemberships(pod *v1.Pod) (sets.String, error) {
	set := sets.String{}

	allCephmons, err := c.cephmonsLister.Cephmons(pod.Namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}



	for i := range allCephmons {
		cephmon := allCephmons[i]

		podName, ok := cephmon.Labels["podName"];
		if !ok {
			continue
		}

		if podName == pod.Name {
			key, err := cache.MetaNamespaceKeyFunc(cephmon)
			if err != nil {
				return nil, err
			}
			set.Insert(key)
		}
	}

	return set, nil
}

func (c *Controller) getCephmonServiceMemberships(cephmon *v1alpha1.Cephmon) (sets.String, error) {
	set := sets.String{}

	var serviceName string
	var ok bool
	if serviceName, ok = cephmon.Labels["service"]; !ok {
		return nil, fmt.Errorf("Unable to get cephmon %v/%v's service memberships: %v",
			cephmon.Namespace, cephmon.Name)
	}

	service, err := c.servicesLister.Services(cephmon.Namespace).Get(serviceName)
	if err != nil {
		return nil, err
	}

	key, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		return nil, err
	}
	set.Insert(key)
	return set, nil
}

