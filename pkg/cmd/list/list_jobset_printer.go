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

package list

import (
	"errors"
	"io"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/utils/clock"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	kueueconstants "sigs.k8s.io/kueue/pkg/controller/constants"

	"sigs.k8s.io/kjob/pkg/constants"
)

type listJobSetPrinter struct {
	clock        clock.Clock
	printOptions printers.PrintOptions
}

var _ printers.ResourcePrinter = (*listJobSetPrinter)(nil)

func (p *listJobSetPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	printer := printers.NewTablePrinter(p.printOptions)

	list, ok := obj.(*jobsetapi.JobSetList)
	if !ok {
		return errors.New("invalid object type")
	}

	table := &metav1.Table{
		ColumnDefinitions: []metav1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name"},
			{Name: "Profile", Type: "string"},
			{Name: "Local Queue", Type: "string"},
		},
		Rows: p.printJobSetList(list),
	}

	return printer.PrintObj(table, out)
}

func (p *listJobSetPrinter) WithNamespace(f bool) *listJobSetPrinter {
	p.printOptions.WithNamespace = f
	return p
}

func (p *listJobSetPrinter) WithHeaders(f bool) *listJobSetPrinter {
	p.printOptions.NoHeaders = !f
	return p
}

func (p *listJobSetPrinter) WithClock(c clock.Clock) *listJobSetPrinter {
	p.clock = c
	return p
}

func newJobSetTablePrinter() *listJobSetPrinter {
	return &listJobSetPrinter{
		clock: clock.RealClock{},
	}
}

func (p *listJobSetPrinter) printJobSetList(list *jobsetapi.JobSetList) []metav1.TableRow {
	rows := make([]metav1.TableRow, len(list.Items))
	for index := range list.Items {
		rows[index] = p.printJobSet(&list.Items[index])
	}
	return rows
}

func (p *listJobSetPrinter) printJobSet(jobSet *jobsetapi.JobSet) metav1.TableRow {
	row := metav1.TableRow{
		Object: runtime.RawExtension{Object: jobSet},
	}
	row.Cells = []any{
		jobSet.Name,
		jobSet.ObjectMeta.Labels[constants.ProfileLabel],
		jobSet.ObjectMeta.Labels[kueueconstants.QueueLabel],
	}
	return row
}
