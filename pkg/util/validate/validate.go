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
	"strings"

	"sigs.k8s.io/kjob/pkg/util"
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

func ValidateMutuallyExclusiveFlags(flagsMap map[string]bool) error {
	if len(flagsMap) < 2 {
		return nil
	}

	allFlags := util.SortedKeys(flagsMap)

	var setFlags []string
	for _, flag := range allFlags {
		if flagsMap[flag] {
			setFlags = append(setFlags, flag)
		}
	}

	if len(setFlags) > 1 {
		return NewMutuallyExclusiveError(allFlags, setFlags)
	}

	return nil
}
