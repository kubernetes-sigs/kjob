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

package create

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/remotecommand"

	"sigs.k8s.io/kjob/apis/v1alpha1"
	kjobctlfake "sigs.k8s.io/kjob/client-go/clientset/versioned/fake"
	cmdtesting "sigs.k8s.io/kjob/pkg/cmd/testing"
	cmdutil "sigs.k8s.io/kjob/pkg/cmd/util"
	"sigs.k8s.io/kjob/pkg/constants"
	"sigs.k8s.io/kjob/pkg/testing/wrappers"
)

func TestCreateOptionsRunInteractive(t *testing.T) {
	testStartTime := time.Now()
	userID := os.Getenv(constants.SystemEnvVarNameUser)

	testCases := map[string]struct {
		options        *CreateOptions
		k8sObjs        []runtime.Object
		kjobctlObjs    []runtime.Object
		createMutation func(pod *corev1.Pod)
		wantPodList    *corev1.PodList
		wantErr        string
	}{
		"success": {
			options: &CreateOptions{
				Namespace:   metav1.NamespaceDefault,
				ProfileName: "profile",
				ModeName:    v1alpha1.InteractiveMode,
				Attach:      &fakeRemoteAttach{},
				AttachFunc:  testAttachFunc,
			},
			k8sObjs: []runtime.Object{
				wrappers.MakePodTemplate("pod-template", metav1.NamespaceDefault).
					WithContainer(*wrappers.MakeContainer("c1", "sleep").Obj()).
					Obj(),
			},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.InteractiveMode, "pod-template").Obj()).
					Obj(),
			},
			createMutation: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
			},
			wantPodList: &corev1.PodList{
				Items: []corev1.Pod{
					*wrappers.MakePod("", metav1.NamespaceDefault).
						GenerateName("profile-interactive-").
						Profile("profile").
						Mode(v1alpha1.InteractiveMode).
						WithContainer(*wrappers.MakeContainer("c1", "sleep").
							TTY().
							Stdin().
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{
								Name:  constants.EnvVarTaskID,
								Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
							}).
							WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)}).
							Obj()).
						Phase(corev1.PodRunning).
						Obj(),
				},
			},
		},
		"success with remove interactive pod": {
			options: &CreateOptions{
				Namespace:    metav1.NamespaceDefault,
				ProfileName:  "profile",
				ModeName:     v1alpha1.InteractiveMode,
				RemoveObject: true,
				Attach:       &fakeRemoteAttach{},
				AttachFunc:   testAttachFunc,
			},
			k8sObjs: []runtime.Object{
				wrappers.MakePodTemplate("pod-template", metav1.NamespaceDefault).
					WithContainer(*wrappers.MakeContainer("c1", "sleep").Obj()).
					Obj(),
			},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.InteractiveMode, "pod-template").Obj()).
					Obj(),
			},
			createMutation: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
			},
			wantPodList: &corev1.PodList{},
		},
		"success with dry-run client": {
			options: &CreateOptions{
				Namespace:      metav1.NamespaceDefault,
				ProfileName:    "profile",
				ModeName:       v1alpha1.InteractiveMode,
				DryRunStrategy: cmdutil.DryRunClient,
				Attach:         &fakeRemoteAttach{},
				AttachFunc:     testAttachFunc,
			},
			k8sObjs: []runtime.Object{
				wrappers.MakePodTemplate("pod-template", metav1.NamespaceDefault).
					WithContainer(*wrappers.MakeContainer("c1", "sleep").Obj()).
					Obj(),
			},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.InteractiveMode, "pod-template").Obj()).
					Obj(),
			},
			wantPodList: &corev1.PodList{},
		},
		"timeout waiting for pod": {
			options: &CreateOptions{
				Namespace:   metav1.NamespaceDefault,
				ProfileName: "profile",
				ModeName:    v1alpha1.InteractiveMode,
				Attach:      &fakeRemoteAttach{},
				AttachFunc:  testAttachFunc,
			},
			k8sObjs: []runtime.Object{
				wrappers.MakePodTemplate("pod-template", metav1.NamespaceDefault).
					WithContainer(*wrappers.MakeContainer("c1", "sleep").Obj()).
					Obj(),
			},
			kjobctlObjs: []runtime.Object{
				wrappers.MakeApplicationProfile("profile", metav1.NamespaceDefault).
					WithSupportedMode(*wrappers.MakeSupportedMode(v1alpha1.InteractiveMode, "pod-template").Obj()).
					Obj(),
			},
			wantPodList: &corev1.PodList{
				Items: []corev1.Pod{
					*wrappers.MakePod("", metav1.NamespaceDefault).
						GenerateName("profile-interactive-").
						Profile("profile").
						Mode(v1alpha1.InteractiveMode).
						WithContainer(*wrappers.MakeContainer("c1", "sleep").
							TTY().
							Stdin().
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarNameUserID, Value: userID}).
							WithEnvVar(corev1.EnvVar{Name: constants.EnvVarTaskName, Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{
								Name:  constants.EnvVarTaskID,
								Value: fmt.Sprintf("%s_%s_default_profile", userID, testStartTime.Format(time.RFC3339)),
							}).
							WithEnvVar(corev1.EnvVar{Name: "PROFILE", Value: "default_profile"}).
							WithEnvVar(corev1.EnvVar{Name: "TIMESTAMP", Value: testStartTime.Format(time.RFC3339)}).
							Obj()).
						Obj(),
				},
			},
			wantErr: "context deadline exceeded",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			streams, _, out, outErr := genericiooptions.NewTestIOStreams()
			tc.options.IOStreams = streams
			tc.options.Out = out
			tc.options.ErrOut = outErr
			tc.options.PrintFlags = genericclioptions.NewPrintFlags("created").WithTypeSetter(k8sscheme.Scheme)
			printer, err := tc.options.PrintFlags.ToPrinter()
			if err != nil {
				t.Fatal(err)
			}
			tc.options.PrintObj = printer.PrintObj

			k8sClientset := k8sfake.NewSimpleClientset(tc.k8sObjs...)
			kjobctlClientset := kjobctlfake.NewSimpleClientset(tc.kjobctlObjs...)
			dynamicClient := fake.NewSimpleDynamicClient(k8sscheme.Scheme)
			restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
			restMapper.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)

			dynamicClient.PrependReactor("create", "pods", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				createAction := action.(kubetesting.CreateAction)

				unstructuredObj := createAction.GetObject().(*unstructured.Unstructured)
				unstructuredObj.SetName(unstructuredObj.GetGenerateName() + utilrand.String(5))

				pod := &corev1.Pod{}

				if err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), pod); err != nil {
					return true, nil, err
				}

				if tc.createMutation != nil {
					tc.createMutation(pod)
				}

				_, err = k8sClientset.CoreV1().Pods(pod.GetNamespace()).Create(t.Context(), pod, metav1.CreateOptions{})
				if err != nil {
					return true, nil, err
				}

				return true, unstructuredObj, err
			})

			tcg := cmdtesting.NewTestClientGetter().
				WithK8sClientset(k8sClientset).
				WithKjobctlClientset(kjobctlClientset).
				WithDynamicClient(dynamicClient).
				WithRESTMapper(restMapper)

			gotErr := tc.options.Run(t.Context(), tcg, testStartTime)

			var gotErrStr string
			if gotErr != nil {
				gotErrStr = gotErr.Error()
			}

			if diff := cmp.Diff(tc.wantErr, gotErrStr); diff != "" {
				t.Errorf("Unexpected error (-want/+got)\n%s", diff)
			}

			gotPodList, err := k8sClientset.CoreV1().Pods(metav1.NamespaceDefault).List(t.Context(), metav1.ListOptions{})
			if err != nil {
				t.Fatal(err)
			}

			defaultCmpOpts := []cmp.Option{cmpopts.IgnoreFields(metav1.ObjectMeta{}, "Name")}
			if diff := cmp.Diff(tc.wantPodList, gotPodList, defaultCmpOpts...); diff != "" {
				t.Errorf("Unexpected error (-want/+got)\n%s", diff)
			}
		})
	}
}

type fakeRemoteAttach struct {
	url *url.URL
	err error
}

func (f *fakeRemoteAttach) Attach(url *url.URL, _ *restclient.Config, _ io.Reader, _, _ io.Writer, _ bool, _ remotecommand.TerminalSizeQueue) error {
	f.url = url
	return f.err
}

func testAttachFunc(o *CreateOptions, _ *corev1.Container, sizeQueue remotecommand.TerminalSizeQueue, _ *corev1.Pod) func() error {
	return func() error {
		u, err := url.Parse("http://kjobctl.test")
		if err != nil {
			return err
		}

		return o.Attach.Attach(u, nil, nil, nil, nil, o.TTY, sizeQueue)
	}
}
