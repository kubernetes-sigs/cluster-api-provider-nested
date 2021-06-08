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
	"crypto"
	"crypto/x509"

	"sigs.k8s.io/cluster-api/util/secret"
)

// KeyPair defines a cert/key pair that is used for the Kubernetes clients
// this was inspired by CAPI's KCP and how it manages CAs.
type KeyPair struct {
	Purpose   secret.Purpose
	Cert      *x509.Certificate
	Key       crypto.Signer
	Generated bool
	New       bool
}

// KeyPairs defines a set of keypairs to act on, this is useful in providing
// helpers to operate on many keypairs.
type KeyPairs []*KeyPair
