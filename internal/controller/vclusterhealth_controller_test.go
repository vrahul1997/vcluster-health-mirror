/*
Copyright 2026.

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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("helper functions", func() {
	Describe("computeScoreLevel", func() {
		It("returns None and 0 when all signals are false", func() {
			score, level := computeScoreLevel(false, false, false, false, false, false)
			Expect(score).To(Equal(int32(0)))
			Expect(level).To(Equal("None"))
		})

		It("returns Partial when some signals are true", func() {
			// 3/6 -> 50
			score, level := computeScoreLevel(true, true, true, false, false, false)
			Expect(score).To(Equal(int32(50)))
			Expect(level).To(Equal("Partial"))
		})

		It("uses integer rounding (5/6 = 83)", func() {
			score, level := computeScoreLevel(true, true, true, true, true, false) // 5/6
			Expect(score).To(Equal(int32(83)))
			Expect(level).To(Equal("Partial"))
		})

		It("returns Full when all signals are true", func() {
			score, level := computeScoreLevel(true, true, true, true, true, true)
			Expect(score).To(Equal(int32(100)))
			Expect(level).To(Equal("Full"))
		})
	})

	Describe("isControlPlaneReady", func() {
		It("returns true when <name>-0 is Running and Ready in the correct namespace", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-0", Namespace: "vcluster"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						}},
					},
				},
			}

			Expect(isControlPlaneReady("vc-prod", "vcluster", pods)).To(BeTrue())
		})

		It("returns false when pod is not Ready", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-0", Namespace: "vcluster"},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						}},
					},
				},
			}

			Expect(isControlPlaneReady("vc-prod", "vcluster", pods)).To(BeFalse())
		})

		It("returns false when pod is in a different namespace", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-0", Namespace: "vcluster-1"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
				},
			}

			Expect(isControlPlaneReady("vc-prod", "vcluster", pods)).To(BeFalse())
		})
	})

	Describe("hasDNSSync", func() {
		It("returns true when kube-dns mapping Service exists in the same namespace", func() {
			svcs := []corev1.Service{
				{ObjectMeta: metav1.ObjectMeta{Name: "kube-dns-x-kube-system-x-vc-prod", Namespace: "vcluster"}},
			}
			Expect(hasDNSSync("vc-prod", "vcluster", svcs)).To(BeTrue())
		})

		It("returns false when kube-dns mapping Service exists but in a different namespace", func() {
			svcs := []corev1.Service{
				{ObjectMeta: metav1.ObjectMeta{Name: "kube-dns-x-kube-system-x-vc-prod", Namespace: "vcluster-1"}},
			}
			Expect(hasDNSSync("vc-prod", "vcluster", svcs)).To(BeFalse())
		})

		It("returns false when no kube-dns mapping Service matches", func() {
			svcs := []corev1.Service{
				{ObjectMeta: metav1.ObjectMeta{Name: "some-other-service", Namespace: "vcluster"}},
			}
			Expect(hasDNSSync("vc-prod", "vcluster", svcs)).To(BeFalse())
		})
	})

	Describe("hasNodeSync", func() {
		It("returns true when node-mapping Service exists in the same namespace", func() {
			svcs := []corev1.Service{
				{ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-node-k3d-k3s-default-server-0", Namespace: "vcluster"}},
			}
			Expect(hasNodeSync("vc-prod", "vcluster", svcs)).To(BeTrue())
		})

		It("returns false when node-mapping Service exists but in a different namespace", func() {
			svcs := []corev1.Service{
				{ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-node-k3d-k3s-default-server-0", Namespace: "vcluster-1"}},
			}
			Expect(hasNodeSync("vc-prod", "vcluster", svcs)).To(BeFalse())
		})

		It("returns false when no node-mapping Service matches", func() {
			svcs := []corev1.Service{
				{ObjectMeta: metav1.ObjectMeta{Name: "vc-prod", Namespace: "vcluster"}},
			}
			Expect(hasNodeSync("vc-prod", "vcluster", svcs)).To(BeFalse())
		})
	})

	Describe("workloadSyncSplit", func() {
		It("returns system=true, tenant=false when only kube-system pods are synced", func() {
			pods := []corev1.Pod{
				// control-plane pod should be ignored
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vc-dev-0",
						Namespace: "vcluster-1",
						Labels:    map[string]string{"app": "vcluster"},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
				// synced system pod
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "coredns-x-kube-system-x-vc-dev",
						Namespace: "vcluster-1",
						Labels: map[string]string{
							"vcluster.loft.sh/managed-by": "vc-dev",
							"vcluster.loft.sh/namespace":  "kube-system",
						},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}

			system, tenant := workloadSyncSplit("vc-dev", "vcluster-1", pods)
			Expect(system).To(BeTrue())
			Expect(tenant).To(BeFalse())
		})

		It("returns tenant=true when a non-kube-system workload is synced", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nginx-x-default-x-vc-prod",
						Namespace: "vcluster",
						Labels: map[string]string{
							"vcluster.loft.sh/managed-by": "vc-prod",
							"vcluster.loft.sh/namespace":  "default",
						},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}

			system, tenant := workloadSyncSplit("vc-prod", "vcluster", pods)
			Expect(system).To(BeFalse())
			Expect(tenant).To(BeTrue())
		})

		It("treats unknown original namespace as tenant (conservative)", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mystery-x-something-x-vc-prod",
						Namespace: "vcluster",
						Labels: map[string]string{
							"vcluster.loft.sh/managed-by": "vc-prod",
							// no vcluster.loft.sh/namespace label
						},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}

			system, tenant := workloadSyncSplit("vc-prod", "vcluster", pods)
			Expect(system).To(BeFalse())
			Expect(tenant).To(BeTrue())
		})
	})

	Describe("hasWorkloadSync (legacy)", func() {
		It("returns false when only control-plane pod exists", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-0", Namespace: "vcluster", Labels: map[string]string{"app": "vcluster"}},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}
			Expect(hasWorkloadSync("vc-prod", "vcluster", pods)).To(BeFalse())
		})

		It("returns true when a synced workload pod has managed-by label", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nginx-x-default-x-vc-prod",
						Namespace: "vcluster",
						Labels: map[string]string{
							"vcluster.loft.sh/managed-by": "vc-prod",
							"vcluster.loft.sh/namespace":  "default",
						},
					},
					Status: corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}
			Expect(hasWorkloadSync("vc-prod", "vcluster", pods)).To(BeTrue())
		})

		It("returns true via namespace-pattern fallback when labels are missing", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "random", Namespace: "team-a-x-vc-prod"},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}
			Expect(hasWorkloadSync("vc-prod", "vcluster", pods)).To(BeTrue())
		})

		It("ignores app=vcluster pods that are not the statefulset pod", func() {
			pods := []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vc-prod-syncer", Namespace: "vcluster", Labels: map[string]string{"app": "vcluster"}},
					Status:     corev1.PodStatus{Phase: corev1.PodRunning},
				},
			}
			Expect(hasWorkloadSync("vc-prod", "vcluster", pods)).To(BeFalse())
		})
	})
})
