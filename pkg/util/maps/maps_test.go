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

package maps

import (
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMergeKeepFirst(t *testing.T) {
	cases := map[string]struct {
		a        map[string]int
		b        map[string]int
		expected map[string]int
	}{
		"a is nil": {
			a:        nil,
			b:        map[string]int{"x": 1, "y": 2},
			expected: map[string]int{"x": 1, "y": 2},
		},
		"b is nil": {
			a:        map[string]int{"x": 1, "y": 2},
			b:        nil,
			expected: map[string]int{"x": 1, "y": 2},
		},
		"no common keys": {
			a:        map[string]int{"x": 1, "y": 2},
			b:        map[string]int{"z": 3, "w": 4},
			expected: map[string]int{"x": 1, "y": 2, "z": 3, "w": 4},
		},
		"common keys, keep values from a": {
			a:        map[string]int{"x": 1, "y": 2, "common": 5},
			b:        map[string]int{"z": 3, "w": 4, "common": 10},
			expected: map[string]int{"x": 1, "y": 2, "z": 3, "w": 4, "common": 5},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var aCopy, bCopy map[string]int
			if tc.a != nil {
				aCopy = maps.Clone(tc.a)
			}
			if tc.b != nil {
				bCopy = maps.Clone(tc.b)
			}

			result := MergeKeepFirst(tc.a, tc.b)

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}

			if tc.a != nil {
				if diff := cmp.Diff(aCopy, tc.a); diff != "" {
					t.Errorf("Original map a was modified (-want,+got):\n%s", diff)
				}
			}

			if tc.b != nil {
				if diff := cmp.Diff(bCopy, tc.b); diff != "" {
					t.Errorf("Original map b was modified (-want,+got):\n%s", diff)
				}
			}
		})
	}
}

func TestSortedKeys(t *testing.T) {
	cases := map[string]struct {
		input    map[string]int
		expected []string
	}{
		"nil map": {
			input:    nil,
			expected: nil,
		},
		"empty map": {
			input:    map[string]int{},
			expected: []string{},
		},
		"single entry": {
			input:    map[string]int{"a": 1},
			expected: []string{"a"},
		},
		"already sorted": {
			input:    map[string]int{"a": 1, "b": 2, "c": 3},
			expected: []string{"a", "b", "c"},
		},
		"not sorted": {
			input:    map[string]int{"c": 3, "a": 1, "b": 2},
			expected: []string{"a", "b", "c"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var inputCopy map[string]int
			if tc.input != nil {
				inputCopy = maps.Clone(tc.input)
			}

			result := SortedKeys(tc.input)

			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("Unexpected result (-want,+got):\n%s", diff)
			}

			if tc.input != nil {
				if diff := cmp.Diff(inputCopy, tc.input); diff != "" {
					t.Errorf("Original map was modified (-want,+got):\n%s", diff)
				}
			}
		})
	}
}
