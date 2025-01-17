// Licensed to Alexandre VILAIN under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Alexandre VILAIN licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package resourceset

import (
	"github.com/alexandrevilain/temporal-operator/api/v1beta1"
	"github.com/alexandrevilain/temporal-operator/pkg/resource"
	"github.com/alexandrevilain/temporal-operator/pkg/resource/mtls/certmanager"
	"github.com/alexandrevilain/temporal-operator/pkg/resource/mtls/istio"
	"github.com/alexandrevilain/temporal-operator/pkg/resource/prometheus"
	"go.temporal.io/server/common/primitives"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClusterBuilder struct {
	Instance *v1beta1.TemporalCluster
	Scheme   *runtime.Scheme
}

func (b *ClusterBuilder) ResourceBuilders() ([]resource.Builder, error) {
	builders := []resource.Builder{
		resource.NewConfigmapBuilder(b.Instance, b.Scheme),
		resource.NewFrontendServiceBuilder(b.Instance, b.Scheme),
	}

	services := []primitives.ServiceName{
		primitives.FrontendService,
		primitives.HistoryService,
		primitives.MatchingService,
		primitives.WorkerService,
	}

	if b.Instance.Spec.Services.InternalFrontend.IsEnabled() {
		services = append(services, primitives.InternalFrontendService)
	}

	for _, service := range services {
		specs, err := b.Instance.Spec.Services.GetServiceSpec(service)
		if err != nil {
			return nil, err
		}

		serviceName := string(service)

		builders = append(builders, resource.NewServiceAccountBuilder(serviceName, b.Instance, b.Scheme, specs))
		builders = append(builders, resource.NewDeploymentBuilder(serviceName, b.Instance, b.Scheme, specs))
		builders = append(builders, resource.NewHeadlessServiceBuilder(serviceName, b.Instance, b.Scheme, specs))

		if b.Instance.Spec.MTLS != nil && b.Instance.Spec.MTLS.Provider == v1beta1.IstioMTLSProvider {
			builders = append(builders, istio.NewPeerAuthenticationBuilder(serviceName, b.Instance, b.Scheme, specs))
			builders = append(builders, istio.NewDestinationRuleBuilder(serviceName, b.Instance, b.Scheme, specs))
		}

		if b.Instance.Spec.Metrics.IsEnabled() &&
			b.Instance.Spec.Metrics.Prometheus != nil &&
			b.Instance.Spec.Metrics.Prometheus.ScrapeConfig != nil &&
			b.Instance.Spec.Metrics.Prometheus.ScrapeConfig.ServiceMonitor != nil &&
			b.Instance.Spec.Metrics.Prometheus.ScrapeConfig.ServiceMonitor.Enabled {
			builders = append(builders, prometheus.NewServiceMonitorBuilder(serviceName, b.Instance, b.Scheme, specs))
		}
	}

	if b.Instance.Spec.DynamicConfig != nil {
		builders = append(builders, resource.NewDynamicConfigmapBuilder(b.Instance, b.Scheme))
	}

	if b.Instance.MTLSWithCertManagerEnabled() {
		builders = append(builders,
			certmanager.NewMTLSBootstrapIssuerBuilder(b.Instance, b.Scheme),
			certmanager.NewMTLSRootCACertificateBuilder(b.Instance, b.Scheme),
			certmanager.NewMTLSRootCAIssuerBuilder(b.Instance, b.Scheme),
		)

		if b.Instance.Spec.MTLS.InternodeEnabled() {
			builders = append(builders,
				certmanager.NewMTLSInternodeIntermediateCACertificateBuilder(b.Instance, b.Scheme),
				certmanager.NewMTLSInternodeIntermediateCAIssuerBuilder(b.Instance, b.Scheme),
				certmanager.NewMTLSInternodeCertificateBuilder(b.Instance, b.Scheme),
			)
		}

		if b.Instance.Spec.MTLS.FrontendEnabled() {
			builders = append(builders,
				certmanager.NewMTLSFrontendIntermediateCACertificateBuilder(b.Instance, b.Scheme),
				certmanager.NewMTLSFrontendIntermediateCAIssuerBuilder(b.Instance, b.Scheme),
				certmanager.NewMTLSFrontendCertificateBuilder(b.Instance, b.Scheme),
			)

			if !b.Instance.Spec.Services.InternalFrontend.IsEnabled() {
				builders = append(builders, certmanager.NewWorkerFrontendClientCertificateBuilder(b.Instance, b.Scheme))
			}
		}
	}

	if b.Instance.Spec.UI != nil && b.Instance.Spec.UI.Enabled {
		builders = append(builders,
			resource.NewUIDeploymentBuilder(b.Instance, b.Scheme),
			resource.NewUIServiceBuilder(b.Instance, b.Scheme),
		)
		if b.Instance.Spec.UI.Ingress != nil {
			builders = append(builders, resource.NewUIIngressBuilder(b.Instance, b.Scheme))
		}

		if b.Instance.MTLSWithCertManagerEnabled() && b.Instance.Spec.MTLS.FrontendEnabled() {
			builders = append(builders, certmanager.NewUIFrontendClientCertificateBuilder(b.Instance, b.Scheme))
		}
	}

	if b.Instance.Spec.AdminTools != nil && b.Instance.Spec.AdminTools.Enabled {
		builders = append(builders, resource.NewAdminToolsDeploymentBuilder(b.Instance, b.Scheme))

		if b.Instance.MTLSWithCertManagerEnabled() && b.Instance.Spec.MTLS.FrontendEnabled() {
			builders = append(builders, certmanager.NewAdminToolsFrontendClientCertificateBuilder(b.Instance, b.Scheme))
		}
	}

	return builders, nil
}

func (b *ClusterBuilder) ResourcePruners() []resource.Pruner {
	pruners := []resource.Pruner{}
	if b.Instance.Spec.UI == nil || (b.Instance.Spec.UI != nil && !b.Instance.Spec.UI.Enabled) {
		pruners = append(pruners, resource.NewUIDeploymentBuilder(b.Instance, b.Scheme))
		pruners = append(pruners, resource.NewUIServiceBuilder(b.Instance, b.Scheme))
		pruners = append(pruners, resource.NewUIIngressBuilder(b.Instance, b.Scheme))
	}
	if b.Instance.Spec.AdminTools == nil || (b.Instance.Spec.AdminTools != nil && !b.Instance.Spec.AdminTools.Enabled) {
		pruners = append(pruners, resource.NewAdminToolsDeploymentBuilder(b.Instance, b.Scheme))
	}
	return pruners
}
