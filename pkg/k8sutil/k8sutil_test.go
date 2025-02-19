// Copyright 2016 The prometheus-operator Authors
//
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

package k8sutil

import (
	"context"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/util/validation"
)

func Test_SanitizeVolumeName(t *testing.T) {
	cases := []struct {
		name     string
		expected string
	}{
		{
			name:     "@$!!@$%!#$%!#$%!#$!#$%%$#@!#",
			expected: "",
		},
		{
			name:     "NAME",
			expected: "name",
		},
		{
			name:     "foo--",
			expected: "foo",
		},
		{
			name:     "foo^%#$bar",
			expected: "foo-bar",
		},
		{
			name:     "fOo^%#$bar",
			expected: "foo-bar",
		},
		{
			name:     strings.Repeat("a", validation.DNS1123LabelMaxLength*2),
			expected: strings.Repeat("a", validation.DNS1123LabelMaxLength),
		},
	}

	for i, c := range cases {
		out := SanitizeVolumeName(c.name)
		if c.expected != out {
			t.Errorf("expected test case %d to be %q but got %q", i, c.expected, out)
		}
	}
}

func TestPropagateKubectlTemplateAnnotations(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name                string
		expectedAnnotations map[string]string
		modifiedAnnotations map[string]string
	}{
		{
			name:                "no change",
			expectedAnnotations: map[string]string{},
		},
		{
			name: "added kubectl annotation",
			expectedAnnotations: map[string]string{
				"kubectl.kubernetes.io/restartedAt": "now",
			},
			modifiedAnnotations: map[string]string{
				"kubectl.kubernetes.io/restartedAt": "now",
				"annotation":                        "value",
			},
		},
		{
			name:                "added another annotation",
			expectedAnnotations: map[string]string{},
			modifiedAnnotations: map[string]string{
				"app.kubernetes.io/name": "overridden-value",
			},
		},
	}

	namespace := "ns-1"

	t.Run("UpdateStatefulSet", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				sset := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "prometheus",
						Namespace: namespace,
					},
				}

				ssetClient := fake.NewSimpleClientset(sset).AppsV1().StatefulSets(namespace)

				modifiedSset := sset.DeepCopy()
				if modifiedSset.Spec.Template.Annotations == nil {
					modifiedSset.Spec.Template.Annotations = make(map[string]string, len(tc.modifiedAnnotations))
				}
				for k, v := range tc.modifiedAnnotations {
					modifiedSset.Spec.Template.Annotations[k] = v
				}

				err := UpdateStatefulSet(ctx, ssetClient, modifiedSset)
				if err != nil {
					t.Fatal(err)
				}

				updatedSset, err := ssetClient.Get(ctx, "prometheus", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(tc.expectedAnnotations, updatedSset.Spec.Template.Annotations) {
					t.Errorf("expected annotations %q, got %q", tc.expectedAnnotations, updatedSset.Spec.Template.Annotations)
				}
			})
		}
	})
}

func TestMergeMetadata(t *testing.T) {
	testCases := []struct {
		name                string
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		modifiedLabels      map[string]string
		modifiedAnnotations map[string]string
	}{
		{
			name: "no change",
			expectedLabels: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
			},
			expectedAnnotations: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
			},
		},
		{
			name: "added label and annotation",
			expectedLabels: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
				"label":                  "value",
			},
			modifiedLabels: map[string]string{
				"label": "value",
			},
			expectedAnnotations: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
				"annotation":             "value",
			},
			modifiedAnnotations: map[string]string{
				"annotation": "value",
			},
		},
		{
			name: "overridden label amd annotation",
			expectedLabels: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
			},
			modifiedLabels: map[string]string{
				"app.kubernetes.io/name": "overridden-value",
			},
			expectedAnnotations: map[string]string{
				"app.kubernetes.io/name": "kube-state-metrics",
			},
			modifiedAnnotations: map[string]string{
				"app.kubernetes.io/name": "overridden-value",
			},
		},
	}

	namespace := "ns-1"

	t.Run("CreateOrUpdateService", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "prometheus-operated",
						Namespace:   namespace,
						Labels:      map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
						Annotations: map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
					},
					Spec:   corev1.ServiceSpec{},
					Status: corev1.ServiceStatus{},
				}

				svcClient := fake.NewSimpleClientset(service).CoreV1().Services(namespace)

				modifiedSvc := service.DeepCopy()
				for l, v := range tc.modifiedLabels {
					modifiedSvc.Labels[l] = v
				}
				for a, v := range tc.modifiedAnnotations {
					modifiedSvc.Annotations[a] = v
				}
				_, err := svcClient.Update(context.Background(), modifiedSvc, metav1.UpdateOptions{})
				if err != nil {
					t.Fatal(err)
				}

				err = CreateOrUpdateService(context.Background(), svcClient, service)
				if err != nil {
					t.Fatal(err)
				}

				updatedSvc, err := svcClient.Get(context.Background(), "prometheus-operated", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(tc.expectedAnnotations, updatedSvc.Annotations) {
					t.Errorf("expected annotations %q, got %q", tc.expectedAnnotations, updatedSvc.Annotations)
				}
				if !reflect.DeepEqual(tc.expectedLabels, updatedSvc.Labels) {
					t.Errorf("expected labels %q, got %q", tc.expectedLabels, updatedSvc.Labels)
				}
			})
		}
	})

	t.Run("CreateOrUpdateEndpoints", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				endpoints := &corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "prometheus-operated",
						Namespace:   namespace,
						Labels:      map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
						Annotations: map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
					},
				}

				endpointsClient := fake.NewSimpleClientset(endpoints).CoreV1().Endpoints(namespace)

				modifiedEndpoints := endpoints.DeepCopy()
				for l, v := range tc.modifiedLabels {
					modifiedEndpoints.Labels[l] = v
				}
				for a, v := range tc.modifiedAnnotations {
					modifiedEndpoints.Annotations[a] = v
				}
				_, err := endpointsClient.Update(context.Background(), modifiedEndpoints, metav1.UpdateOptions{})
				if err != nil {
					t.Fatal(err)
				}

				err = CreateOrUpdateEndpoints(context.Background(), endpointsClient, endpoints)
				if err != nil {
					t.Fatal(err)
				}

				updatedEndpoints, err := endpointsClient.Get(context.Background(), "prometheus-operated", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(tc.expectedAnnotations, updatedEndpoints.Annotations) {
					t.Errorf("expected annotations %q, got %q", tc.expectedAnnotations, updatedEndpoints.Annotations)
				}
				if !reflect.DeepEqual(tc.expectedLabels, updatedEndpoints.Labels) {
					t.Errorf("expected labels %q, got %q", tc.expectedLabels, updatedEndpoints.Labels)
				}
			})
		}
	})

	t.Run("UpdateStatefulSet", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				sset := &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "prometheus",
						Namespace:   namespace,
						Labels:      map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
						Annotations: map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
					},
				}

				ssetClient := fake.NewSimpleClientset(sset).AppsV1().StatefulSets(namespace)

				modifiedSset := sset.DeepCopy()
				for l, v := range tc.modifiedLabels {
					modifiedSset.Labels[l] = v
				}
				for a, v := range tc.modifiedAnnotations {
					modifiedSset.Annotations[a] = v
				}
				_, err := ssetClient.Update(context.Background(), modifiedSset, metav1.UpdateOptions{})
				if err != nil {
					t.Fatal(err)
				}

				err = UpdateStatefulSet(context.Background(), ssetClient, sset)
				if err != nil {
					t.Fatal(err)
				}

				updatedSset, err := ssetClient.Get(context.Background(), "prometheus", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(tc.expectedAnnotations, updatedSset.Annotations) {
					t.Errorf("expected annotations %q, got %q", tc.expectedAnnotations, updatedSset.Annotations)
				}
				if !reflect.DeepEqual(tc.expectedLabels, updatedSset.Labels) {
					t.Errorf("expected labels %q, got %q", tc.expectedLabels, updatedSset.Labels)
				}
			})
		}
	})

	t.Run("CreateOrUpdateSecret", func(t *testing.T) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "prometheus-tls-assets",
						Namespace:   namespace,
						Labels:      map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
						Annotations: map[string]string{"app.kubernetes.io/name": "kube-state-metrics"},
					},
				}

				sClient := fake.NewSimpleClientset(secret).CoreV1().Secrets(namespace)

				modifiedSecret := secret.DeepCopy()
				for l, v := range tc.modifiedLabels {
					modifiedSecret.Labels[l] = v
				}
				for a, v := range tc.modifiedAnnotations {
					modifiedSecret.Annotations[a] = v
				}
				_, err := sClient.Update(context.Background(), modifiedSecret, metav1.UpdateOptions{})
				if err != nil {
					t.Fatal(err)
				}

				err = CreateOrUpdateSecret(context.Background(), sClient, secret)
				if err != nil {
					t.Fatal(err)
				}

				updatedSecret, err := sClient.Get(context.Background(), "prometheus-tls-assets", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}

				if !reflect.DeepEqual(tc.expectedAnnotations, updatedSecret.Annotations) {
					t.Errorf("expected annotations %q, got %q", tc.expectedAnnotations, updatedSecret.Annotations)
				}
				if !reflect.DeepEqual(tc.expectedLabels, updatedSecret.Labels) {
					t.Errorf("expected labels %q, got %q", tc.expectedLabels, updatedSecret.Labels)
				}
			})
		}
	})
}

func TestCreateOrUpdateImmutableFields(t *testing.T) {
	namespace := "default"
	policy := corev1.IPFamilyPolicyRequireDualStack

	t.Run("CreateOrUpdateService with immutable fields", func(t *testing.T) {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-operated-test",
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: "127.0.0.1",
				ClusterIPs: []string{
					"127.0.0.1",
					"192.168.0.159",
				},
				IPFamilyPolicy: &policy,
				IPFamilies: []corev1.IPFamily{
					corev1.IPv6Protocol,
				},
				Ports: []corev1.ServicePort{
					{
						Name: "https-metrics",
						Port: 10250,
					},
					{
						Name: "http-metrics",
						Port: 10255,
					},
				},
			},
			Status: corev1.ServiceStatus{},
		}

		svcClient := fake.NewSimpleClientset(service).CoreV1().Services(namespace)

		modifiedSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-operated-test",
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "https-metrics",
						Port: 10250,
					},
				},
			},
			Status: corev1.ServiceStatus{},
		}

		if err := CreateOrUpdateService(context.TODO(), svcClient, modifiedSvc); err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(service.Spec.IPFamilies, modifiedSvc.Spec.IPFamilies) {
			t.Fatalf("services Spec.IPFamilies are not equal, expected %q, got %q",
				service.Spec.IPFamilies, modifiedSvc.Spec.IPFamilies)
		}

		if !reflect.DeepEqual(service.Spec.ClusterIP, modifiedSvc.Spec.ClusterIP) {
			t.Fatalf("services Spec.ClusterIP are not equal, expected %q, got %q",
				service.Spec.ClusterIP, modifiedSvc.Spec.ClusterIP)
		}

		if !reflect.DeepEqual(service.Spec.ClusterIPs, modifiedSvc.Spec.ClusterIPs) {
			t.Fatalf("services Spec.ClusterIPs are not equal, expected %q, got %q",
				service.Spec.ClusterIPs, modifiedSvc.Spec.ClusterIPs)
		}

		if !reflect.DeepEqual(service.Spec.IPFamilyPolicy, modifiedSvc.Spec.IPFamilyPolicy) {
			t.Fatalf("services Spec.IPFamilyPolicy are not equal, expected %v, got %v",
				service.Spec.IPFamilyPolicy, modifiedSvc.Spec.IPFamilyPolicy)
		}

		if !reflect.DeepEqual(service.Spec.IPFamilyPolicy, modifiedSvc.Spec.IPFamilyPolicy) {
			t.Fatalf("services Spec.IPFamilyPolicy are not equal, expected %v, got %v",
				service.Spec.IPFamilyPolicy, modifiedSvc.Spec.IPFamilyPolicy)
		}
	})
}
