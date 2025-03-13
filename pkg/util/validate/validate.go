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

package validate

import (
	"fmt"
	"sort"
	"strings"
)

type MutuallyExclusiveError string

func (e MutuallyExclusiveError) Error() string {
	return string(e)
}

func NewMutuallyExclusiveError(allFlags, setFlags []string) MutuallyExclusiveError {
	return MutuallyExclusiveError(
		fmt.Sprintf(
			"if any flags in the group [%s] are set none of the others can be; [%s] were set",
			strings.Join(allFlags, " "),
			strings.Join(setFlags, " "),
		),
	)
}

func ValidateMutuallyExclusiveFlags(flags map[string]bool) error {
	if len(flags) < 2 {
		return nil
	}

	var (
		setFlags = make([]string, 0)
		allFlags = make([]string, 0, len(flags))
	)

	for f, isSet := range flags {
		allFlags = append(allFlags, f)
		if isSet {
			setFlags = append(setFlags, f)
		}
	}

	sort.Strings(setFlags)
	sort.Strings(allFlags)

	if len(setFlags) > 1 {
		return NewMutuallyExclusiveError(allFlags, setFlags)
	}

	return nil
}
