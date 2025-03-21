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

package kjobctl

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	testingclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetapi "sigs.k8s.io/jobset/api/jobset/v1alpha2"
	kueueconstants "sigs.k8s.io/kueue/pkg/controller/constants"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	"sigs.k8s.io/kjob/pkg/cmd"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
	"sigs.k8s.io/kjob/test/helpers"
)

var _ = ginkgo.Describe("Kjobctl Create", ginkgo.Ordered, ginkgo.ContinueOnFailure, func() {
	var ns *corev1.Namespace

	ginkgo.BeforeEach(func() {
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "ns-"}}
		gomega.Expect(k8sClient.Create(ctx, ns)).To(gomega.Succeed())
	})

	ginkgo.AfterEach(func() {
		gomega.Expect(helpers.DeleteNamespace(ctx, k8sClient, ns)).To(gomega.Succeed())
	})

	ginkgo.When("Create a JobSet", func() {
		var (
			jobTemplate *v1alpha1.JobSetTemplate
			profile     *v1alpha1.ApplicationProfile
		)

		ginkgo.BeforeEach(func() {
			exampleJobSetSpec := jobsetapi.JobSetSpec{
				ReplicatedJobs: []jobsetapi.ReplicatedJob{
					{
						Name: "test",
						Template: batchv1.JobTemplateSpec{
							Spec: batchv1.JobSpec{
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Name:  "c",
												Image: "busybox",
											},
										},
									},
								},
							},
						},
					},
				},
			}
			jobTemplate = wrappers.MakeJobSetTemplate("jobset-template", ns.Name).
				Spec(exampleJobSetSpec).
				Obj()
			gomega.Expect(k8sClient.Create(ctx, jobTemplate)).To(gomega.Succeed())

			profile = wrappers.MakeApplicationProfile("profile", ns.Name).
				WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.JobSetMode, "jobset-template").Obj()).
				Obj()
			gomega.Expect(k8sClient.Create(ctx, profile)).To(gomega.Succeed())
		})

		ginkgo.It("Should create jobSet", func() {
			testStartTime := time.Now()

			ginkgo.By("Create a JobSet", func() {
				streams, _, out, outErr := genericiooptions.NewTestIOStreams()
				configFlags := CreateConfigFlagsWithRestConfig(cfg, streams)

				kjobctlCmd := cmd.NewKjobctlCmd(cmd.KjobctlOptions{
					ConfigFlags: configFlags,
					IOStreams:   streams,
					Clock:       testingclock.NewFakeClock(testStartTime),
				})
				kjobctlCmd.SetOut(out)
				kjobctlCmd.SetErr(outErr)
				kjobctlCmd.SetArgs([]string{
					"create", "jobset",
					"-n", ns.Name,
					"--profile", profile.Name,
					"--localqueue", "lq1",
					"--skip-localqueue-validation",
				})

				err := kjobctlCmd.Execute()
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "%s: %s", err, out)
				gomega.Expect(outErr.String()).Should(gomega.BeEmpty())
				gomega.Expect(out.String()).Should(gomega.MatchRegexp(
					fmt.Sprintf("^jobset.jobset.x-k8s.io/%s-%s-[a-zA-Z0-9]+ created\\n$", profile.Name, "jobset")))
			})

			ginkgo.By("Check that JobSet created", func() {
				jobSetList := &jobsetapi.JobSetList{}
				gomega.Expect(k8sClient.List(ctx, jobSetList, client.InNamespace(ns.Name))).To(gomega.Succeed())
				gomega.Expect(jobSetList.Items).To(gomega.HaveLen(1))
				gomega.Expect(jobSetList.Items[0].Labels[constants.ProfileLabel]).To(gomega.Equal(profile.Name))
				gomega.Expect(jobSetList.Items[0].Labels[kueueconstants.QueueLabel]).To(gomega.Equal("lq1"))
			})
		})

		// SimpleDynamicClient didn't allow to check server dry run flag.
		ginkgo.It("Shouldn't create jobSet with server dry run", func() {
			ginkgo.By("Create a JobSet", func() {
				streams, _, out, outErr := genericiooptions.NewTestIOStreams()
				configFlags := CreateConfigFlagsWithRestConfig(cfg, streams)

				kjobctlCmd := cmd.NewKjobctlCmd(cmd.KjobctlOptions{ConfigFlags: configFlags, IOStreams: streams})
				kjobctlCmd.SetOut(out)
				kjobctlCmd.SetErr(outErr)
				kjobctlCmd.SetArgs([]string{"create", "jobset", "-n", ns.Name, "--profile", profile.Name, "--dry-run", "server"})

				err := kjobctlCmd.Execute()
				gomega.Expect(err).NotTo(gomega.HaveOccurred(), "%s: %s", err, out)
				gomega.Expect(outErr.String()).Should(gomega.BeEmpty())
				gomega.Expect(out.String()).Should(gomega.MatchRegexp(
					fmt.Sprintf("jobset.jobset.x-k8s.io/%s-%s-[a-zA-Z0-9]+ created \\(server dry run\\)", profile.Name, "jobset")))
			})

			ginkgo.By("Check that JobSet not created", func() {
				jobSetList := &jobsetapi.JobSetList{}
				gomega.Expect(k8sClient.List(ctx, jobSetList, client.InNamespace(ns.Name))).To(gomega.Succeed())
				gomega.Expect(jobSetList.Items).To(gomega.BeEmpty())
			})
		})
	})
})
