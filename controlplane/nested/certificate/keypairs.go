/*
Copyright 2021 The Kubernetes Authors.
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

package certificate

import (
	"context"

	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Lookup looks up each certificate from secrets and populates the certificate with the secret data.
func (kp KeyPairs) Lookup(ctx context.Context, cli client.Client, clusterName client.ObjectKey) error {
	// Look up each certificate as a secret and populate the certificate/key
	for _, certificate := range kp {
		s := &corev1.Secret{}
		key := client.ObjectKey{
			Name:      secret.Name(clusterName.Name, certificate.Purpose),
			Namespace: clusterName.Namespace,
		}
		if err := cli.Get(ctx, key, s); err != nil {
			if apierrors.IsNotFound(err) {
				certificate.New = true
				continue
			}
			return errors.WithStack(err)
		}
		certificate.New = false
	}
	return nil
}

// SaveGenerated will save any certificates that have been generated as Kubernetes secrets.
func (kp KeyPairs) SaveGenerated(ctx context.Context, ctrlclient client.Client, clusterName client.ObjectKey, owner metav1.OwnerReference) error {
	for _, keyPair := range kp {
		if !keyPair.Generated && !keyPair.New {
			continue
		}
		s := keyPair.AsSecret(clusterName, owner)
		if err := ctrlclient.Create(ctx, s); err != nil {
			// We might want to trigger off updates from here.
			if apierrors.IsAlreadyExists(err) {
				return nil
			}
			return err
		}
	}
	return nil
}

// LookupOrGenerate is a convenience function that wraps cluster bootstrap certificate behavior.
func (kp KeyPairs) LookupOrSave(ctx context.Context, ctrlclient client.Client, clusterName client.ObjectKey, owner metav1.OwnerReference) error {
	// Find the certificates that exist
	if err := kp.Lookup(ctx, ctrlclient, clusterName); err != nil {
		return err
	}

	// Save any certificates that have been generated
	if err := kp.SaveGenerated(ctx, ctrlclient, clusterName, owner); err != nil {
		return err
	}

	return nil
}
