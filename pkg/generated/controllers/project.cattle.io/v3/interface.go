/*
Copyright 2024 Rancher Labs, Inc.

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
	"github.com/rancher/lasso/pkg/controller"
	v3 "github.com/rancher/rancher/pkg/apis/project.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/schemes"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func init() {
	schemes.Register(v3.AddToScheme)
}

type Interface interface {
	BasicAuth() BasicAuthController
	Certificate() CertificateController
	DockerCredential() DockerCredentialController
	NamespacedBasicAuth() NamespacedBasicAuthController
	NamespacedCertificate() NamespacedCertificateController
	NamespacedDockerCredential() NamespacedDockerCredentialController
	NamespacedSSHAuth() NamespacedSSHAuthController
	NamespacedServiceAccountToken() NamespacedServiceAccountTokenController
	SSHAuth() SSHAuthController
	ServiceAccountToken() ServiceAccountTokenController
	Workload() WorkloadController
}

func New(controllerFactory controller.SharedControllerFactory) Interface {
	return &version{
		controllerFactory: controllerFactory,
	}
}

type version struct {
	controllerFactory controller.SharedControllerFactory
}

func (v *version) BasicAuth() BasicAuthController {
	return generic.NewController[*v3.BasicAuth, *v3.BasicAuthList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "BasicAuth"}, "basicauths", true, v.controllerFactory)
}

func (v *version) Certificate() CertificateController {
	return generic.NewController[*v3.Certificate, *v3.CertificateList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "Certificate"}, "certificates", true, v.controllerFactory)
}

func (v *version) DockerCredential() DockerCredentialController {
	return generic.NewController[*v3.DockerCredential, *v3.DockerCredentialList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "DockerCredential"}, "dockercredentials", true, v.controllerFactory)
}

func (v *version) NamespacedBasicAuth() NamespacedBasicAuthController {
	return generic.NewController[*v3.NamespacedBasicAuth, *v3.NamespacedBasicAuthList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "NamespacedBasicAuth"}, "namespacedbasicauths", true, v.controllerFactory)
}

func (v *version) NamespacedCertificate() NamespacedCertificateController {
	return generic.NewController[*v3.NamespacedCertificate, *v3.NamespacedCertificateList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "NamespacedCertificate"}, "namespacedcertificates", true, v.controllerFactory)
}

func (v *version) NamespacedDockerCredential() NamespacedDockerCredentialController {
	return generic.NewController[*v3.NamespacedDockerCredential, *v3.NamespacedDockerCredentialList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "NamespacedDockerCredential"}, "namespaceddockercredentials", true, v.controllerFactory)
}

func (v *version) NamespacedSSHAuth() NamespacedSSHAuthController {
	return generic.NewController[*v3.NamespacedSSHAuth, *v3.NamespacedSSHAuthList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "NamespacedSSHAuth"}, "namespacedsshauths", true, v.controllerFactory)
}

func (v *version) NamespacedServiceAccountToken() NamespacedServiceAccountTokenController {
	return generic.NewController[*v3.NamespacedServiceAccountToken, *v3.NamespacedServiceAccountTokenList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "NamespacedServiceAccountToken"}, "namespacedserviceaccounttokens", true, v.controllerFactory)
}

func (v *version) SSHAuth() SSHAuthController {
	return generic.NewController[*v3.SSHAuth, *v3.SSHAuthList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "SSHAuth"}, "sshauths", true, v.controllerFactory)
}

func (v *version) ServiceAccountToken() ServiceAccountTokenController {
	return generic.NewController[*v3.ServiceAccountToken, *v3.ServiceAccountTokenList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "ServiceAccountToken"}, "serviceaccounttokens", true, v.controllerFactory)
}

func (v *version) Workload() WorkloadController {
	return generic.NewController[*v3.Workload, *v3.WorkloadList](schema.GroupVersionKind{Group: "project.cattle.io", Version: "v3", Kind: "Workload"}, "workloads", true, v.controllerFactory)
}
