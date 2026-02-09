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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DiscoveredCluster represents a vCluster discovered in the host cluster.
type DiscoveredCluster struct {
	// Name is the vCluster name (usually the Service name).
	Name string `json:"name"`
	// Namespace is the host namespace where the vCluster Service lives.
	Namespace string `json:"namespace"`
	// ServiceName is the Kubernetes Service name backing the vCluster API endpoint.
	ServiceName string `json:"serviceName"`
	// ServicePort is the API port exposed by the Service (typically 443).
	ServicePort int32 `json:"servicePort"`
}

// SyncCoverage summarizes which vCluster sync features are active (host-side signals only).
type SyncCoverage struct {
	// ClusterName is the vCluster name (e.g., vc-prod).
	ClusterName string `json:"clusterName"`

	// ControlPlaneReady indicates the vCluster control-plane pod (vc-prod-0) is running & ready.
	ControlPlaneReady bool `json:"controlPlaneReady"`

	// ApiSync indicates the vCluster API Service exists (vc-prod).
	ApiSync bool `json:"apiSync"`

	// DnsSync indicates kube-system DNS mapping Service exists (kube-dns-x-*-x-vc-prod).
	DnsSync bool `json:"dnsSync"`

	// NodeSync indicates node-mapping Services exist (vc-prod-node-*).
	NodeSync bool `json:"nodeSync"`

	// WorkloadSync is a legacy aggregate. It is true if either SystemWorkloadSync or TenantWorkloadSync is true.
	WorkloadSync bool `json:"workloadSync"`

	// SystemWorkloadSync is true if kube-system workloads (e.g. CoreDNS) are observed synced on the host.
	SystemWorkloadSync bool `json:"systemWorkloadSync"`

	// TenantWorkloadSync is true if non-kube-system tenant workloads are observed synced on the host.
	TenantWorkloadSync bool `json:"tenantWorkloadSync"`

	// Score is a simple percentage (0–100) derived from the signals above.
	Score int32 `json:"score"`

	// Level is a human-friendly summary: None | Partial | Full.
	Level string `json:"level"`

	// LastChecked is when this coverage was last evaluated.
	// +optional
	LastChecked metav1.Time `json:"lastChecked,omitempty"`
}

// VClusterHealthSpec defines the desired state of VClusterHealth
type VClusterHealthSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// IntervalSeconds controls how often the controller re-checks.
	// If 0, defaults to 30 seconds.
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`
	// Namespace is the host namespace where vCluster Services live (defaults to "vcluster" if empty).
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// SyncCoverage reports host-observed sync signals per vCluster.
	// +optional
	SyncCoverage []SyncCoverage `json:"syncCoverage,omitempty"`
}

// VClusterHealthStatus defines the observed state of VClusterHealth.
type VClusterHealthStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// LastUpdated is when this status was last refreshed.
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the VClusterHealth resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Clusters is the list of discovered vCluster entrypoints (host cluster Services).
	// +optional
	Clusters []DiscoveredCluster `json:"clusters,omitempty"`

	// SyncCoverage reports host-observed sync signals per vCluster.
	// +optional
	SyncCoverage []SyncCoverage `json:"syncCoverage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// VClusterHealth is the Schema for the vclusterhealths API
// adding print column using kube builder for additional fields
// +kubebuilder:printcolumn:name="TargetNS",type="string",JSONPath=".spec.namespace",description="Namespace selector (vcluster, *, all)"
// +kubebuilder:printcolumn:name="Score",type="integer",JSONPath=".status.syncCoverage[0].score",description="Score (first entry)"
// +kubebuilder:printcolumn:name="Level",type="string",JSONPath=".status.syncCoverage[0].level",description="Level (first entry)"
// +kubebuilder:printcolumn:name="SysWL",type="boolean",JSONPath=".status.syncCoverage[0].systemWorkloadSync",description="System workload sync (first entry)"
// +kubebuilder:printcolumn:name="TenantWL",type="boolean",JSONPath=".status.syncCoverage[0].tenantWorkloadSync",description="Tenant workload sync (first entry)"
// +kubebuilder:printcolumn:name="LastUpdated",type="date",JSONPath=".status.lastUpdated",description="Last status update"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type VClusterHealth struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of VClusterHealth
	// +required
	Spec VClusterHealthSpec `json:"spec"`

	// status defines the observed state of VClusterHealth
	// +optional
	Status VClusterHealthStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// VClusterHealthList contains a list of VClusterHealth
type VClusterHealthList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []VClusterHealth `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VClusterHealth{}, &VClusterHealthList{})
}
