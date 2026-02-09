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
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	fleetv1alpha1 "github.com/vrahul1997/vcluster-health-mirror/api/v1alpha1"
)

// VClusterHealthReconciler reconciles a VClusterHealth object
type VClusterHealthReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=fleet.health.io,resources=vclusterhealths,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=fleet.health.io,resources=vclusterhealths/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=fleet.health.io,resources=vclusterhealths/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VClusterHealth object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
func (r *VClusterHealthReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// _ = logf.FromContext(ctx) This line is not needed, we'll add our own logger.
	logger := log.FromContext(ctx)
	// Get an empty Vcluster Health CR
	var vh fleetv1alpha1.VClusterHealth

	// Error block for getting the object (default/fleet)
	if err := r.Get(ctx, req.NamespacedName, &vh); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Interval compute ==> Get the inyterval from CR and multply it by second
	interval := time.Duration(vh.Spec.IntervalSeconds) * time.Second
	// Interval minimum set to 30 seconds
	if interval <= 0 {
		interval = 30 * time.Second
	}

	logger.Info("loaded vCluster", "name", req.NamespacedName.String(), "next", interval.String())

	// Namespace selection:
	// - default: "vcluster"
	// - "*" or "all": discover vClusters across all namespaces
	targetNS := vh.Spec.Namespace
	if targetNS == "" {
		targetNS = "vcluster"
	}
	allNamespaces := targetNS == "*" || targetNS == "all"

	var svcList corev1.ServiceList

	// List vCluster API Services (app=vcluster). If allNamespaces is enabled, list cluster-wide.
	svcListOpts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{"app": "vcluster"}),
	}
	if !allNamespaces {
		svcListOpts.Namespace = targetNS
	}
	if err := r.List(ctx, &svcList, svcListOpts); err != nil {
		logger.Error(err, "failed to list vcluster services", "namespace", targetNS, "allNamespaces", allNamespaces)
		return ctrl.Result{RequeueAfter: interval}, nil
	}
	// log the discovered services in the current namespace, future change it to all the available namespaces.
	logger.Info("discovered services", "namespace", targetNS, "allNamespaces", allNamespaces, "count", len(svcList.Items))

	// List all Services cluster-wide. We'll filter by namespace when evaluating DNS/Node signals.
	var allSvcList corev1.ServiceList
	if err := r.List(ctx, &allSvcList, &client.ListOptions{}); err != nil {
		logger.Error(err, "failed to list all services")
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	// List pods cluster-wide. We'll filter by namespace for control-plane readiness.
	var podList corev1.PodList
	if err := r.List(ctx, &podList, &client.ListOptions{}); err != nil {
		logger.Error(err, "failed to list pods")
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	// List pods across all namespaces to detect synced workloads.
	// Workloads synced from the vCluster usually live outside the vcluster control-plane namespace.
	var allPodList corev1.PodList
	if err := r.List(ctx, &allPodList, &client.ListOptions{}); err != nil {
		logger.Error(err, "failed to list all pods")
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	discovered := make([]fleetv1alpha1.DiscoveredCluster, 0, len(svcList.Items))

	for _, s := range svcList.Items {
		// We have some headless helper services that doesnt have a cluster ip, we will omit them with this block
		if s.Spec.ClusterIP == corev1.ClusterIPNone {
			continue
		}
		// Pick port 443 if present, otherwise fall back to the first port.
		var port int32 = 443
		if len(s.Spec.Ports) > 0 {
			port = s.Spec.Ports[0].Port
			for _, p := range s.Spec.Ports {
				if p.Port == 443 {
					port = 443
					break
				}
			}
		}

		discovered = append(discovered, fleetv1alpha1.DiscoveredCluster{
			Name:        s.Name,
			Namespace:   s.Namespace,
			ServiceName: s.Name,
			ServicePort: port,
		})

	}

	logger.Info("built discovered cluster", "count", len(discovered))

	// ---- Sync Coverage (API sync only, initial step) ----
	syncCoverage := make([]fleetv1alpha1.SyncCoverage, 0, len(discovered))
	now := v1.Now()

	for _, c := range discovered {
		// we able to discover the cluster, only because API server was working, so defaults to true
		api := true
		cp := isControlPlaneReady(c.Name, c.Namespace, podList.Items)
		dns := hasDNSSync(c.Name, c.Namespace, allSvcList.Items)
		node := hasNodeSync(c.Name, c.Namespace, allSvcList.Items)

		// For workload detection, the namespace to treat as "control plane" is the vCluster's namespace.
		sysWL, tenantWL := workloadSyncSplit(c.Name, c.Namespace, allPodList.Items)
		wl := sysWL || tenantWL // legacy aggregate

		score, level := computeScoreLevel(api, cp, dns, node, sysWL, tenantWL)

		// If we discovered the API Service for a vCluster, API sync is considered present.
		syncCoverage = append(syncCoverage, fleetv1alpha1.SyncCoverage{
			ClusterName:        c.Name,
			ApiSync:            api,
			ControlPlaneReady:  cp,
			DnsSync:            dns,
			NodeSync:           node,
			WorkloadSync:       wl,
			SystemWorkloadSync: sysWL,
			TenantWorkloadSync: tenantWL,
			Score:              score,
			Level:              level,
			LastChecked:        now,
		})
	}

	vh.Status.Clusters = discovered
	vh.Status.SyncCoverage = syncCoverage
	if err := r.Status().Update(ctx, &vh); err != nil {
		logger.Error(err, "failed to update VclusterHealth status")
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	logger.Info("updated status.clusters", "count", len(vh.Status.Clusters))
	logger.Info("updated status.syncCoverage", "count", len(vh.Status.SyncCoverage))

	return ctrl.Result{RequeueAfter: interval}, nil
}

// isControlPlaneReady returns true if the vCluster control-plane pod (<name>-0) in the given namespace is Running and Ready.
func isControlPlaneReady(vclusterName, namespace string, pods []corev1.Pod) bool {
	target := vclusterName + "-0"
	for _, p := range pods {
		if p.Namespace != namespace {
			continue
		}
		if p.Name != target {
			continue
		}
		if p.Status.Phase != corev1.PodRunning {
			return false
		}
		for _, c := range p.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}
	return false
}

// hasDNSSync returns true if a kube-dns mapping Service exists for the vCluster in the given namespace.
// Example: kube-dns-x-kube-system-x-vc-prod
func hasDNSSync(vclusterName, namespace string, services []corev1.Service) bool {
	for _, s := range services {
		if s.Namespace != namespace {
			continue
		}
		if strings.Contains(s.Name, "kube-dns") && strings.Contains(s.Name, vclusterName) {
			return true
		}
	}
	return false
}

// hasNodeSync returns true if node-mapping Services exist for the vCluster in the given namespace.
// Example: vc-prod-node-k3d-k3s-default-server-0
func hasNodeSync(vclusterName, namespace string, services []corev1.Service) bool {
	needle := vclusterName + "-node-"
	for _, s := range services {
		if s.Namespace != namespace {
			continue
		}
		if strings.Contains(s.Name, needle) {
			return true
		}
	}
	return false
}

//  1. Label-based: pods created by the syncer often carry a vCluster label whose value == vclusterName.
//  2. Namespace-name fallback: some setups create namespaces that contain "-x-<vclusterName>".
//
// IMPORTANT: Do NOT skip the entire control-plane namespace. In many setups (including yours),
// synced workload pods live in the same namespace as the vCluster control plane (e.g. nginx-x-default-x-vc-prod in namespace vcluster).
// Instead, we skip only true control-plane pods (app=vcluster) and the StatefulSet pod (<name>-0).
func hasWorkloadSync(vclusterName, controlPlaneNamespace string, pods []corev1.Pod) bool {
	// Common labels used by vCluster for synced objects (varies by version/config).
	labelKeys := []string{
		"vcluster.loft.sh/managed-by",
		"vcluster.loft.sh/vcluster-name",
		"vcluster.loft.sh/cluster",
		"vcluster.loft.sh/owner",
	}

	nsNeedle := "-x-" + vclusterName
	controlPlanePod := vclusterName + "-0"

	for _, p := range pods {
		// Skip control-plane namespace entirely; we only care about synced workloads.
		if p.Namespace == controlPlaneNamespace && p.Name == controlPlanePod {
			continue
		}

		// Skip other control-plane components (they usually carry app=vcluster).
		if p.Labels != nil {
			if app, ok := p.Labels["app"]; ok && app == "vcluster" {
				continue
			}
		}

		// 1) Label-based detection.
		if p.Labels != nil {
			for _, k := range labelKeys {
				if v, ok := p.Labels[k]; ok && v == vclusterName {
					return true
				}
			}
		}

		// 2) Namespace-pattern fallback.
		if strings.Contains(p.Namespace, nsNeedle) {
			return true
		}
	}

	return false
}

// workloadSyncSplit returns (systemWorkload, tenantWorkload).
// We use the vCluster-provided label `vcluster.loft.sh/namespace` to understand the ORIGINAL namespace inside the vCluster.
// - systemWorkload: any synced pod whose original namespace == kube-system
// - tenantWorkload: any synced pod whose original namespace != kube-system
// Control-plane pods (app=vcluster) and the StatefulSet pod (<name>-0) are excluded.
func workloadSyncSplit(vclusterName, controlPlaneNamespace string, pods []corev1.Pod) (bool, bool) {
	labelKeys := []string{
		"vcluster.loft.sh/managed-by",
		"vcluster.loft.sh/vcluster-name",
		"vcluster.loft.sh/cluster",
		"vcluster.loft.sh/owner",
	}

	controlPlanePod := vclusterName + "-0"
	system := false
	tenant := false

	for _, p := range pods {
		// Exclude the control-plane StatefulSet pod.
		if p.Namespace == controlPlaneNamespace && p.Name == controlPlanePod {
			continue
		}
		// Exclude other control-plane components.
		if p.Labels != nil {
			if app, ok := p.Labels["app"]; ok && app == "vcluster" {
				continue
			}
		}

		// Determine whether this pod belongs to this vCluster (label-based detection).
		belongs := false
		if p.Labels != nil {
			for _, k := range labelKeys {
				if v, ok := p.Labels[k]; ok && v == vclusterName {
					belongs = true
					break
				}
			}
		}
		if !belongs {
			continue
		}

		origNS := ""
		if p.Labels != nil {
			origNS = p.Labels["vcluster.loft.sh/namespace"]
		}

		if origNS == "kube-system" {
			system = true
		} else {
			// If we can't determine origNS, we conservatively treat it as tenant.
			tenant = true
		}

		if system && tenant {
			return true, true
		}
	}

	return system, tenant
}

// computeScoreLevel converts sync booleans into a simple percentage score and a human-friendly level.
// total signals: API, ControlPlaneReady, DNS, Node, SystemWorkload, TenantWorkload (6).
// - Score: 0..100
// - Level: None (0/6), Partial (1-5/6), Full (6/6)
func computeScoreLevel(api, controlPlane, dns, node, systemWorkload, tenantWorkload bool) (int32, string) {
	total := int32(6)
	points := int32(0)
	if api {
		points++
	}
	if controlPlane {
		points++
	}
	if dns {
		points++
	}
	if node {
		points++
	}
	if systemWorkload {
		points++
	}
	if tenantWorkload {
		points++
	}

	// integer percentage
	score := (points * 100) / total

	level := "None"
	if points == total {
		level = "Full"
	} else if points > 0 {
		level = "Partial"
	}

	return score, level
}

// SetupWithManager sets up the controller with the Manager.
func (r *VClusterHealthReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&fleetv1alpha1.VClusterHealth{}).
		Named("vclusterhealth").
		Complete(r)
}
