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

package parse

import (
	// Import the hash implementations or this package will panic if
	// Contour or Envoy images reference a sha hash. See the following
	// for details: https://github.com/opencontainers/go-digest#usage
	_ "crypto/sha256"
	_ "crypto/sha512"
	"fmt"

	"github.com/docker/distribution/reference"
)

// Image parses s, returning and error if s is not a syntactically
// valid image reference. Image does not not handle short digests.
func Image(s string) error {
	_, err := reference.Parse(s)
	if err != nil {
		return fmt.Errorf("failed to parse s %s: %w", s, err)
	}

	return nil
}
