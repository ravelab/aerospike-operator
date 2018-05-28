/*
Copyright 2018 The aerospike-operator Authors.

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

package e2e

import (
	"flag"
	"testing"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/travelaudience/aerospike-operator/test/e2e/framework"
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to the kubeconfig file to be used")
	flag.StringVar(&framework.GCSBucketName, "gcs-bucket-name", "", "the name of the GCS bucket to be used to store backups")
	flag.StringVar(&framework.GCSSecretName, "gcs-secret-name", "", "the name of the secret containing the credentials to access the GCS bucket")
	flag.Parse()
}

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
