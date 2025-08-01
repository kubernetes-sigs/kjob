/*
Copyright 2024 The Kubernetes Authors.

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

package parser

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
)

var (
	errInvalidArrayFlagFormat = errors.New("invalid array flag format")
)

var (
	commaIndexesRegex = regexp.MustCompile(`^[0-9]\d*(,[1-9]\d*)*$`)
	rangeIndexesRegex = regexp.MustCompile(`(^[0-9]\d*-[1-9]\d*)(([:%])([1-9]\d*))?$`)
)

type ArrayIndexes struct {
	Indexes     []int32
	Step        *int32
	Parallelism *int32
}

func (ai ArrayIndexes) Count() int {
	return len(ai.Indexes)
}

func (ai ArrayIndexes) Min() int32 {
	return slices.Min(ai.Indexes)
}

func (ai ArrayIndexes) Max() int32 {
	return slices.Max(ai.Indexes)
}

// ParseArrayIndexes parse array flag to ArrayIndexes.
// Include syntax like:
//   - 1-5   - which results in indexes: 1, 2, 3, 4, 5
//   - 1,4,5 - which results exactly in the mentioned indexes 1, 4, 5
//   - 3-9:3 - with step indicator, which results in 3,6,9
//   - 1-5%2 - which results in indexes: 1, 2, 3, 4, 5 but only 2 of them are processed at the same time.
func ParseArrayIndexes(str string) (*ArrayIndexes, error) {
	arrayIndexes := &ArrayIndexes{}

	var (
		indexes     []int32
		step        *int32
		parallelism *int32
		err         error
	)

	if commaIndexesRegex.MatchString(str) {
		indexes, err = parseCommaSeparatedIndexes(str)
	} else if matches := rangeIndexesRegex.FindStringSubmatch(str); matches != nil {
		var num int64

		if matches[4] != "" {
			num, err = strconv.ParseInt(matches[4], 10, 32)
			if err != nil {
				return nil, errInvalidArrayFlagFormat
			}
		}

		switch matches[3] {
		case ":":
			step = ptr.To(int32(num))
		case "%":
			parallelism = ptr.To(int32(num))
		}

		indexes, err = parseRangeIndexes(matches[1], step)
	} else {
		return nil, errInvalidArrayFlagFormat
	}

	if err != nil {
		return nil, err
	}

	slices.Sort(indexes)

	arrayIndexes.Indexes = indexes
	arrayIndexes.Step = step
	arrayIndexes.Parallelism = parallelism

	return arrayIndexes, nil
}

func parseRangeIndexes(str string, step *int32) ([]int32, error) {
	if step == nil {
		step = ptr.To[int32](1)
	}

	if *step <= 0 {
		return nil, errInvalidArrayFlagFormat
	}

	parts := strings.Split(str, "-")
	if len(parts) != 2 {
		return nil, errInvalidArrayFlagFormat
	}

	from, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return nil, errInvalidArrayFlagFormat
	}

	to, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return nil, errInvalidArrayFlagFormat
	}

	if from > to {
		return nil, errInvalidArrayFlagFormat
	}

	var indexes []int32

	for i := int32(from); i <= int32(to); i += *step {
		indexes = append(indexes, i)
	}

	return indexes, nil
}

func parseCommaSeparatedIndexes(str string) ([]int32, error) {
	strIndexes := strings.Split(str, ",")
	maxValue := int32(-1)
	set := sets.New[int32]()
	for _, strIndex := range strIndexes {
		num, err := strconv.ParseInt(strIndex, 10, 32)
		if err != nil {
			return nil, errInvalidArrayFlagFormat
		}
		index := int32(num)
		if index < maxValue {
			return nil, errInvalidArrayFlagFormat
		}
		if set.Has(index) {
			return nil, errInvalidArrayFlagFormat
		}
		set.Insert(index)
		maxValue = index
	}
	return set.UnsortedList(), nil
}
