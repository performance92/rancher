/*
Copyright 2019 Rancher Labs, Inc.

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

// Code generated by main. DO NOT EDIT.

package v3

import (
	"context"
	"time"

	clientset "github.com/rancher/rancher/pkg/wrangler/generated/clientset/versioned/typed/management.cattle.io/v3"
	informers "github.com/rancher/rancher/pkg/wrangler/generated/informers/externalversions/management.cattle.io/v3"
	listers "github.com/rancher/rancher/pkg/wrangler/generated/listers/management.cattle.io/v3"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type ClusterHandler func(string, *v3.Cluster) (*v3.Cluster, error)

type ClusterController interface {
	generic.ControllerMeta
	ClusterClient

	OnChange(ctx context.Context, name string, sync ClusterHandler)
	OnRemove(ctx context.Context, name string, sync ClusterHandler)
	Enqueue(name string)
	EnqueueAfter(name string, duration time.Duration)

	Cache() ClusterCache
}

type ClusterClient interface {
	Create(*v3.Cluster) (*v3.Cluster, error)
	Update(*v3.Cluster) (*v3.Cluster, error)
	UpdateStatus(*v3.Cluster) (*v3.Cluster, error)
	Delete(name string, options *metav1.DeleteOptions) error
	Get(name string, options metav1.GetOptions) (*v3.Cluster, error)
	List(opts metav1.ListOptions) (*v3.ClusterList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.Cluster, err error)
}

type ClusterCache interface {
	Get(name string) (*v3.Cluster, error)
	List(selector labels.Selector) ([]*v3.Cluster, error)

	AddIndexer(indexName string, indexer ClusterIndexer)
	GetByIndex(indexName, key string) ([]*v3.Cluster, error)
}

type ClusterIndexer func(obj *v3.Cluster) ([]string, error)

type clusterController struct {
	controllerManager *generic.ControllerManager
	clientGetter      clientset.ClustersGetter
	informer          informers.ClusterInformer
	gvk               schema.GroupVersionKind
}

func NewClusterController(gvk schema.GroupVersionKind, controllerManager *generic.ControllerManager, clientGetter clientset.ClustersGetter, informer informers.ClusterInformer) ClusterController {
	return &clusterController{
		controllerManager: controllerManager,
		clientGetter:      clientGetter,
		informer:          informer,
		gvk:               gvk,
	}
}

func FromClusterHandlerToHandler(sync ClusterHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v3.Cluster
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v3.Cluster))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *clusterController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v3.Cluster))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateClusterDeepCopyOnChange(client ClusterClient, obj *v3.Cluster, handler func(obj *v3.Cluster) (*v3.Cluster, error)) (*v3.Cluster, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *clusterController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controllerManager.AddHandler(ctx, c.gvk, c.informer.Informer(), name, handler)
}

func (c *clusterController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	removeHandler := generic.NewRemoveHandler(name, c.Updater(), handler)
	c.controllerManager.AddHandler(ctx, c.gvk, c.informer.Informer(), name, removeHandler)
}

func (c *clusterController) OnChange(ctx context.Context, name string, sync ClusterHandler) {
	c.AddGenericHandler(ctx, name, FromClusterHandlerToHandler(sync))
}

func (c *clusterController) OnRemove(ctx context.Context, name string, sync ClusterHandler) {
	removeHandler := generic.NewRemoveHandler(name, c.Updater(), FromClusterHandlerToHandler(sync))
	c.AddGenericHandler(ctx, name, removeHandler)
}

func (c *clusterController) Enqueue(name string) {
	c.controllerManager.Enqueue(c.gvk, c.informer.Informer(), "", name)
}

func (c *clusterController) EnqueueAfter(name string, duration time.Duration) {
	c.controllerManager.EnqueueAfter(c.gvk, c.informer.Informer(), "", name, duration)
}

func (c *clusterController) Informer() cache.SharedIndexInformer {
	return c.informer.Informer()
}

func (c *clusterController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *clusterController) Cache() ClusterCache {
	return &clusterCache{
		lister:  c.informer.Lister(),
		indexer: c.informer.Informer().GetIndexer(),
	}
}

func (c *clusterController) Create(obj *v3.Cluster) (*v3.Cluster, error) {
	return c.clientGetter.Clusters().Create(obj)
}

func (c *clusterController) Update(obj *v3.Cluster) (*v3.Cluster, error) {
	return c.clientGetter.Clusters().Update(obj)
}

func (c *clusterController) UpdateStatus(obj *v3.Cluster) (*v3.Cluster, error) {
	return c.clientGetter.Clusters().UpdateStatus(obj)
}

func (c *clusterController) Delete(name string, options *metav1.DeleteOptions) error {
	return c.clientGetter.Clusters().Delete(name, options)
}

func (c *clusterController) Get(name string, options metav1.GetOptions) (*v3.Cluster, error) {
	return c.clientGetter.Clusters().Get(name, options)
}

func (c *clusterController) List(opts metav1.ListOptions) (*v3.ClusterList, error) {
	return c.clientGetter.Clusters().List(opts)
}

func (c *clusterController) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.clientGetter.Clusters().Watch(opts)
}

func (c *clusterController) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v3.Cluster, err error) {
	return c.clientGetter.Clusters().Patch(name, pt, data, subresources...)
}

type clusterCache struct {
	lister  listers.ClusterLister
	indexer cache.Indexer
}

func (c *clusterCache) Get(name string) (*v3.Cluster, error) {
	return c.lister.Get(name)
}

func (c *clusterCache) List(selector labels.Selector) ([]*v3.Cluster, error) {
	return c.lister.List(selector)
}

func (c *clusterCache) AddIndexer(indexName string, indexer ClusterIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v3.Cluster))
		},
	}))
}

func (c *clusterCache) GetByIndex(indexName, key string) (result []*v3.Cluster, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		result = append(result, obj.(*v3.Cluster))
	}
	return result, nil
}

type ClusterStatusHandler func(obj *v3.Cluster, status v3.ClusterStatus) (v3.ClusterStatus, error)

type ClusterGeneratingHandler func(obj *v3.Cluster, status v3.ClusterStatus) ([]runtime.Object, v3.ClusterStatus, error)

func RegisterClusterStatusHandler(ctx context.Context, controller ClusterController, condition condition.Cond, name string, handler ClusterStatusHandler) {
	statusHandler := &clusterStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromClusterHandlerToHandler(statusHandler.sync))
}

func RegisterClusterGeneratingHandler(ctx context.Context, controller ClusterController, apply apply.Apply,
	condition condition.Cond, name string, handler ClusterGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &clusterGeneratingHandler{
		ClusterGeneratingHandler: handler,
		apply:                    apply,
		name:                     name,
		gvk:                      controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	RegisterClusterStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type clusterStatusHandler struct {
	client    ClusterClient
	condition condition.Cond
	handler   ClusterStatusHandler
}

func (a *clusterStatusHandler) sync(key string, obj *v3.Cluster) (*v3.Cluster, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	obj.Status = newStatus
	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(obj, "", nil)
		} else {
			a.condition.SetError(obj, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, obj.Status) {
		var newErr error
		obj, newErr = a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
	}
	return obj, err
}

type clusterGeneratingHandler struct {
	ClusterGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *clusterGeneratingHandler) Handle(obj *v3.Cluster, status v3.ClusterStatus) (v3.ClusterStatus, error) {
	objs, newStatus, err := a.ClusterGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	apply := a.apply

	if !a.opts.DynamicLookup {
		apply = apply.WithStrictCaching()
	}

	if !a.opts.AllowCrossNamespace && !a.opts.AllowClusterScoped {
		apply = apply.WithSetOwnerReference(true, false).
			WithDefaultNamespace(obj.GetNamespace()).
			WithListerNamespace(obj.GetNamespace())
	}

	if !a.opts.AllowClusterScoped {
		apply = apply.WithRestrictClusterScoped()
	}

	return newStatus, apply.
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
