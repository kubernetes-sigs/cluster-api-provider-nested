/*
Copyright 2019 The Kubernetes Authors.

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

package server

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"

	"github.com/emicklei/go-restful"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/cmd/vn-agent/app/options"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/vn-agent/config"
)

// Server is a http.Handler which exposes vn-agent functionality over HTTP.
type Server struct {
	config                *config.Config
	restfulCont           *restful.Container
	transport             *http.Transport
	superAPIServerAddress *url.URL
	restConfig            *rest.Config
	enableMetrics         bool
}

// ServeHTTP responds to HTTP requests on the vn-agent.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.restfulCont.ServeHTTP(w, req)
}

// NewServer initializes and configures a vn-agent.Server object to handle HTTP requests.
func NewServer(cfg *config.Config, serverOption *options.ServerOption) (*Server, error) {
	u, err := url.Parse(cfg.KubeletServerHost)
	if err != nil {
		return nil, errors.Wrap(err, "parse kubelet server url")
	}
	cfg.KubeletServerHost = u.Host

	server := &Server{
		restfulCont:   restful.NewContainer(),
		config:        cfg,
		enableMetrics: serverOption.EnableMetrics,
	}

	server.InstallHandlers()

	if server.config.KubeletClientCert != nil {
		server.transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
				Certificates:       []tls.Certificate{*server.config.KubeletClientCert},
			},
		}
	} else {
		var restConfig *rest.Config
		var caCrtPool *x509.CertPool
		if len(serverOption.Kubeconfig) == 0 {
			restConfig, err = rest.InClusterConfig()
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get in cluster config")
			}
			caCrtPool, err = certutil.NewPool(restConfig.TLSClientConfig.CAFile)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get cert pool")
			}
		} else {
			// This creates a client, first loading any specified kubeconfig\
			restConfig, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
				&clientcmd.ClientConfigLoadingRules{ExplicitPath: serverOption.Kubeconfig},
				&clientcmd.ConfigOverrides{}).ClientConfig()
			if err != nil {
				return nil, errors.Wrapf(err, "failed to construct client config")
			}
			caCrtPool, err = certutil.NewPoolFromBytes(restConfig.TLSClientConfig.CAData)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get cert pool")
			}
		}
		server.restConfig = restConfig
		superHTTPSURL, err := url.Parse(restConfig.Host)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse apiserver address")
		}
		server.superAPIServerAddress = superHTTPSURL
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse ca file")
		}
		server.transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCrtPool,
				MinVersion: tls.VersionTLS12,
			},
		}
	}

	return server, nil
}
