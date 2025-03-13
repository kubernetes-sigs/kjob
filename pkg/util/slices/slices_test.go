/*
Copyright 2025 The Kubernetes Authors.

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

package slices

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMap(t *testing.T) {
	testCases := map[string]struct {
		slice    []int
		f        func(from *int) string
		expected []string
	}{
		"empty slice": {},
		"slice": {
			slice: []int{1, 2, 3},
			f: func(from *int) string {
				return strconv.Itoa(*from)
			},
			expected: []string{"1", "2", "3"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := Map(tc.slice, tc.f)
			if diff := cmp.Diff(tc.expected, got); diff != "" {
				t.Errorf("Map result (-want, +got): %s", diff)
			}
		})
	}
}
