// Copyright 2018 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"fmt"
	"io"

	"github.com/operator-framework/operator-sdk/pkg/handler"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	cached "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ genericclioptions.RESTClientGetter = &restClientGetter{}

type restClientGetter struct {
	restConfig      *rest.Config
	discoveryClient discovery.CachedDiscoveryInterface
	restMapper      meta.RESTMapper
	namespaceConfig clientcmd.ClientConfig
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.discoveryClient, nil
}

func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return c.restMapper, nil
}

func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return c.namespaceConfig
}

var _ clientcmd.ClientConfig = &namespaceClientConfig{}

type namespaceClientConfig struct {
	namespace string
}

func (c namespaceClientConfig) RawConfig() (clientcmdapi.Config, error) {
	return clientcmdapi.Config{}, nil
}

func (c namespaceClientConfig) ClientConfig() (*rest.Config, error) {
	return nil, nil
}

func (c namespaceClientConfig) Namespace() (string, bool, error) {
	return c.namespace, false, nil
}

func (c namespaceClientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return nil
}

// NewRESTClientGetter ...
func NewRESTClientGetter(mgr manager.Manager, ns string) (genericclioptions.RESTClientGetter, error) {
	cfg := mgr.GetConfig()
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	cdc := cached.NewMemCacheClient(dc)
	rm := mgr.GetRESTMapper()

	return &restClientGetter{
		restConfig:      cfg,
		discoveryClient: cdc,
		restMapper:      rm,
		namespaceConfig: &namespaceClientConfig{ns},
	}, nil
}

var _ kube.Interface = &ownerRefInjectingClient{}

func NewOwnerRefInjectingClient(base kube.Client, ownerRef metav1.OwnerReference,
	mgr manager.Manager, cr *unstructured.Unstructured) kube.Interface {
	return &ownerRefInjectingClient{
		refs:       []metav1.OwnerReference{ownerRef},
		Client:     base,
		restMapper: mgr.GetRESTMapper(),
		owner:      cr,
	}
}

type ownerRefInjectingClient struct {
	refs []metav1.OwnerReference
	kube.Client
	restMapper meta.RESTMapper
	owner      *unstructured.Unstructured
}

func (c *ownerRefInjectingClient) Build(reader io.Reader, validate bool) (kube.ResourceList, error) {
	resourceList, err := c.Client.Build(reader, validate)
	if err != nil {
		return resourceList, err
	}
	err = resourceList.Visit(func(r *resource.Info, err error) error {
		if err != nil {
			return err
		}
		objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r.Object)
		if err != nil {
			return err
		}
		u := &unstructured.Unstructured{Object: objMap}
		useOwnerRef, err := SupportsOwnerReference(c.restMapper, c.owner, u)
		if err != nil {
			return err
		}

		if useOwnerRef {
			u.SetOwnerReferences(c.refs)
		} else {
			a := u.GetAnnotations()
			if a == nil {
				a = map[string]string{}
			}
			a[handler.NamespacedNameAnnotation] = fmt.Sprintf("%s/%s", c.owner.GetNamespace(), c.owner.GetName())
			a[handler.TypeAnnotation] = c.owner.GetObjectKind().GroupVersionKind().GroupKind().String()

			u.SetAnnotations(a)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resourceList, nil
}

func SupportsOwnerReference(restMapper meta.RESTMapper, owner, dependent runtime.Object) (bool, error) {
	ownerGVK := owner.GetObjectKind().GroupVersionKind()
	ownerMapping, err := restMapper.RESTMapping(ownerGVK.GroupKind(), ownerGVK.Version)
	if err != nil {
		return false, err
	}
	mOwner, err := meta.Accessor(owner)
	if err != nil {
		return false, err
	}

	depGVK := dependent.GetObjectKind().GroupVersionKind()
	depMapping, err := restMapper.RESTMapping(depGVK.GroupKind(), depGVK.Version)
	if err != nil {
		return false, err
	}
	mDep, err := meta.Accessor(dependent)
	if err != nil {
		return false, err
	}

	ownerClusterScoped := ownerMapping.Scope.Name() == meta.RESTScopeNameRoot
	ownerNamespace := mOwner.GetNamespace()
	depClusterScoped := depMapping.Scope.Name() == meta.RESTScopeNameRoot
	depNamespace := mDep.GetNamespace()

	if ownerClusterScoped {
		return true, nil
	}

	if depClusterScoped {
		return false, nil
	}

	if ownerNamespace != depNamespace {
		return false, nil
	}

	return true, nil
}
