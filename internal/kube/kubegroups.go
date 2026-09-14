package kube

// This file registers every built in Kubernetes API group version. The
// OpenShift scheme installer covers most of them but not all, and a snapshot
// can hold objects written by an older API server, so the full set is
// registered to keep protobuf decoding complete.

import (
	"k8s.io/apimachinery/pkg/runtime"

	kadmissionv1 "k8s.io/api/admission/v1"
	kadmissionv1beta1 "k8s.io/api/admission/v1beta1"
	kadmissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	kadmissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	kadmissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	kapidiscoveryv2 "k8s.io/api/apidiscovery/v2"
	kapidiscoveryv2beta1 "k8s.io/api/apidiscovery/v2beta1"
	kapiserverinternalv1alpha1 "k8s.io/api/apiserverinternal/v1alpha1"
	kappsv1 "k8s.io/api/apps/v1"
	kappsv1beta1 "k8s.io/api/apps/v1beta1"
	kappsv1beta2 "k8s.io/api/apps/v1beta2"
	kauthenticationv1 "k8s.io/api/authentication/v1"
	kauthenticationv1alpha1 "k8s.io/api/authentication/v1alpha1"
	kauthenticationv1beta1 "k8s.io/api/authentication/v1beta1"
	kauthorizationv1 "k8s.io/api/authorization/v1"
	kauthorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	kautoscalingv1 "k8s.io/api/autoscaling/v1"
	kautoscalingv2 "k8s.io/api/autoscaling/v2"
	kbatchv1 "k8s.io/api/batch/v1"
	kbatchv1beta1 "k8s.io/api/batch/v1beta1"
	kcertificatesv1 "k8s.io/api/certificates/v1"
	kcertificatesv1alpha1 "k8s.io/api/certificates/v1alpha1"
	kcertificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	kcoordinationv1 "k8s.io/api/coordination/v1"
	kcoordinationv1alpha2 "k8s.io/api/coordination/v1alpha2"
	kcoordinationv1beta1 "k8s.io/api/coordination/v1beta1"
	kcorev1 "k8s.io/api/core/v1"
	kdiscoveryv1 "k8s.io/api/discovery/v1"
	kdiscoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	keventsv1 "k8s.io/api/events/v1"
	keventsv1beta1 "k8s.io/api/events/v1beta1"
	kextensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	kflowcontrolv1 "k8s.io/api/flowcontrol/v1"
	kflowcontrolv1beta1 "k8s.io/api/flowcontrol/v1beta1"
	kflowcontrolv1beta2 "k8s.io/api/flowcontrol/v1beta2"
	kflowcontrolv1beta3 "k8s.io/api/flowcontrol/v1beta3"
	kimagepolicyv1alpha1 "k8s.io/api/imagepolicy/v1alpha1"
	knetworkingv1 "k8s.io/api/networking/v1"
	knetworkingv1beta1 "k8s.io/api/networking/v1beta1"
	knodev1 "k8s.io/api/node/v1"
	knodev1alpha1 "k8s.io/api/node/v1alpha1"
	knodev1beta1 "k8s.io/api/node/v1beta1"
	kpolicyv1 "k8s.io/api/policy/v1"
	kpolicyv1beta1 "k8s.io/api/policy/v1beta1"
	krbacv1 "k8s.io/api/rbac/v1"
	krbacv1alpha1 "k8s.io/api/rbac/v1alpha1"
	krbacv1beta1 "k8s.io/api/rbac/v1beta1"
	kresourcev1 "k8s.io/api/resource/v1"
	kresourcev1alpha3 "k8s.io/api/resource/v1alpha3"
	kresourcev1beta1 "k8s.io/api/resource/v1beta1"
	kresourcev1beta2 "k8s.io/api/resource/v1beta2"
	kschedulingv1 "k8s.io/api/scheduling/v1"
	kschedulingv1alpha2 "k8s.io/api/scheduling/v1alpha2"
	kschedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	kstoragev1 "k8s.io/api/storage/v1"
	kstoragev1alpha1 "k8s.io/api/storage/v1alpha1"
	kstoragev1beta1 "k8s.io/api/storage/v1beta1"
	kstoragemigrationv1beta1 "k8s.io/api/storagemigration/v1beta1"
)

var kubeGroups = runtime.NewSchemeBuilder(
	kadmissionv1.AddToScheme,
	kadmissionv1beta1.AddToScheme,
	kadmissionregistrationv1.AddToScheme,
	kadmissionregistrationv1alpha1.AddToScheme,
	kadmissionregistrationv1beta1.AddToScheme,
	kapidiscoveryv2.AddToScheme,
	kapidiscoveryv2beta1.AddToScheme,
	kapiserverinternalv1alpha1.AddToScheme,
	kappsv1.AddToScheme,
	kappsv1beta1.AddToScheme,
	kappsv1beta2.AddToScheme,
	kauthenticationv1.AddToScheme,
	kauthenticationv1alpha1.AddToScheme,
	kauthenticationv1beta1.AddToScheme,
	kauthorizationv1.AddToScheme,
	kauthorizationv1beta1.AddToScheme,
	kautoscalingv1.AddToScheme,
	kautoscalingv2.AddToScheme,
	kbatchv1.AddToScheme,
	kbatchv1beta1.AddToScheme,
	kcertificatesv1.AddToScheme,
	kcertificatesv1alpha1.AddToScheme,
	kcertificatesv1beta1.AddToScheme,
	kcoordinationv1.AddToScheme,
	kcoordinationv1alpha2.AddToScheme,
	kcoordinationv1beta1.AddToScheme,
	kcorev1.AddToScheme,
	kdiscoveryv1.AddToScheme,
	kdiscoveryv1beta1.AddToScheme,
	keventsv1.AddToScheme,
	keventsv1beta1.AddToScheme,
	kextensionsv1beta1.AddToScheme,
	kflowcontrolv1.AddToScheme,
	kflowcontrolv1beta1.AddToScheme,
	kflowcontrolv1beta2.AddToScheme,
	kflowcontrolv1beta3.AddToScheme,
	kimagepolicyv1alpha1.AddToScheme,
	knetworkingv1.AddToScheme,
	knetworkingv1beta1.AddToScheme,
	knodev1.AddToScheme,
	knodev1alpha1.AddToScheme,
	knodev1beta1.AddToScheme,
	kpolicyv1.AddToScheme,
	kpolicyv1beta1.AddToScheme,
	krbacv1.AddToScheme,
	krbacv1alpha1.AddToScheme,
	krbacv1beta1.AddToScheme,
	kresourcev1.AddToScheme,
	kresourcev1alpha3.AddToScheme,
	kresourcev1beta1.AddToScheme,
	kresourcev1beta2.AddToScheme,
	kschedulingv1.AddToScheme,
	kschedulingv1alpha2.AddToScheme,
	kschedulingv1beta1.AddToScheme,
	kstoragev1.AddToScheme,
	kstoragev1alpha1.AddToScheme,
	kstoragev1beta1.AddToScheme,
	kstoragemigrationv1beta1.AddToScheme,
)
