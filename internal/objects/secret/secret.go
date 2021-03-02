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

package secret

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Get returns a Secret resource named ns/name, if it exists.
func Get(ctx context.Context, cli client.Client, ns, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: ns,
		Name:      name,
	}
	if err := cli.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", ns, name, err)
	}
	return secret, nil
}
