// Copyright Project Contour Authors
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

	"github.com/projectcontour/contour-operator/internal/operator"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func New() (client.Client, error) {
	scheme := operator.GetOperatorScheme()
	opts := client.Options{
		Scheme: scheme,
	}
	cli, err := client.New(config.GetConfigOrDie(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create operator client: %w", err)
	}
	return cli, nil
}
