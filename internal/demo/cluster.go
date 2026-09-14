package demo

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mjovanovic0/openshift-etcd-backup-explorer/internal/kube"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// The keyspace prefixes the OpenShift API server writes under.
const (
	kubePrefix      = "/kubernetes.io"
	openshiftPrefix = "/openshift.io"
)

// namespaces are deliberately generic. A real backup holds real customer
// names, which is exactly why this fixture exists.
var namespaces = []string{
	"openshift-etcd",
	"openshift-monitoring",
	"openshift-apiserver",
	"openshift-ingress",
	"kube-system",
	"demo-shop",
	"demo-analytics",
	"demo-staging",
}

// workloads describes the applications the demo cluster runs.
var workloads = []struct {
	namespace string
	app       string
	replicas  int32
	port      int32
}{
	{"openshift-monitoring", "alertmanager", 3, 9093},
	{"openshift-monitoring", "prometheus", 3, 9090},
	{"openshift-apiserver", "apiserver", 3, 8443},
	{"openshift-ingress", "router-default", 3, 443},
	{"demo-shop", "storefront", 4, 8080},
	{"demo-shop", "checkout", 3, 8080},
	{"demo-analytics", "ingest", 3, 9000},
	{"demo-staging", "storefront", 2, 8080},
}

func nsTime(offsetDays int) metav1.Time {
	return metav1.NewTime(TakenAt.Add(-time.Duration(offsetDays) * 24 * time.Hour).Truncate(time.Second))
}

// buildCluster produces every object the demo backup holds, already encoded
// the way the API server stores it.
func buildCluster() ([]record, error) {
	b := &builder{}

	b.namespaces()
	b.persistentVolumes()
	b.rbac()
	b.etcdConfigMaps()
	b.etcdStaticPods()
	for i, w := range workloads {
		b.workload(i, w.namespace, w.app, w.replicas, w.port)
	}
	b.customResources()

	return b.records, b.err
}

type builder struct {
	records []record
	err     error
}

// addProto stores an object the way built in and OpenShift types are stored.
func (b *builder) addProto(key string, obj runtime.Object, apiVersion, kind string) {
	if b.err != nil {
		return
	}
	value, err := kube.EncodeProtobuf(obj, apiVersion, kind)
	if err != nil {
		b.err = err
		return
	}
	b.records = append(b.records, record{key: key, value: value})
}

// addJSON stores an object the way a custom resource is stored.
func (b *builder) addJSON(key string, doc map[string]any) {
	if b.err != nil {
		return
	}
	value, err := json.Marshal(doc)
	if err != nil {
		b.err = err
		return
	}
	b.records = append(b.records, record{key: key, value: value})
}

func meta(name, namespace string, ageDays int) metav1.ObjectMeta {
	m := metav1.ObjectMeta{
		Name:              name,
		Namespace:         namespace,
		UID:               types(name, namespace),
		ResourceVersion:   fmt.Sprintf("%d", 1000000+len(name)*37+ageDays*11),
		CreationTimestamp: nsTime(ageDays),
		// managedFields is the bulk of a real object and the main thing the
		// export strips, so the fixture must carry it.
		ManagedFields: []metav1.ManagedFieldsEntry{{
			Manager:    "kube-controller-manager",
			Operation:  metav1.ManagedFieldsOperationUpdate,
			APIVersion: "v1",
			FieldsType: "FieldsV1",
			Time:       &[]metav1.Time{nsTime(ageDays)}[0],
		}},
	}
	return m
}

// types builds a stable fake UID from the object's identity, so regenerating
// the fixture gives the same result.
func types(name, namespace string) apitypesUID {
	h := uint64(1469598103934665603)
	for _, c := range []byte(namespace + "/" + name) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return apitypesUID(fmt.Sprintf("%08x-%04x-4%03x-8%03x-%012x",
		uint32(h>>32), uint16(h>>16), uint16(h)&0xfff, uint16(h>>4)&0xfff, h&0xffffffffffff))
}

func (b *builder) namespaces() {
	for i, ns := range namespaces {
		obj := &corev1.Namespace{
			ObjectMeta: meta(ns, "", 400-i*7),
			Spec:       corev1.NamespaceSpec{Finalizers: []corev1.FinalizerName{corev1.FinalizerKubernetes}},
			Status:     corev1.NamespaceStatus{Phase: corev1.NamespaceActive},
		}
		obj.Labels = map[string]string{"kubernetes.io/metadata.name": ns}
		b.addProto(kubePrefix+"/namespaces/"+ns, obj, "v1", "Namespace")
	}
}

func (b *builder) persistentVolumes() {
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("pv-demo-%03d", i)
		claimNS := namespaces[5+i%3]
		pv := &corev1.PersistentVolume{
			ObjectMeta: meta(name, "", 120-i*3),
			Spec: corev1.PersistentVolumeSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", 5+i*5))},
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					NFS: &corev1.NFSVolumeSource{Server: "nfs.demo.example.com", Path: "/exports/" + name},
				},
				ClaimRef: &corev1.ObjectReference{
					Kind: "PersistentVolumeClaim", Namespace: claimNS,
					Name: fmt.Sprintf("data-%03d", i), UID: apitypesUID(types(fmt.Sprintf("data-%03d", i), claimNS)),
					APIVersion: "v1",
				},
				StorageClassName:              "nfs",
				PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			},
			Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
		}
		pv.Finalizers = []string{"kubernetes.io/pv-protection"}
		b.addProto(kubePrefix+"/persistentvolumes/"+name, pv, "v1", "PersistentVolume")

		pvc := &corev1.PersistentVolumeClaim{
			ObjectMeta: meta(fmt.Sprintf("data-%03d", i), claimNS, 120-i*3),
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				VolumeName:  name,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", 5+i*5))},
				},
			},
			Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		}
		b.addProto(fmt.Sprintf("%s/persistentvolumeclaims/%s/data-%03d", kubePrefix, claimNS, i), pvc, "v1", "PersistentVolumeClaim")
	}
}

func (b *builder) rbac() {
	for i, ns := range namespaces {
		sa := &corev1.ServiceAccount{ObjectMeta: meta("builder", ns, 380-i*5)}
		b.addProto(kubePrefix+"/serviceaccounts/"+ns+"/builder", sa, "v1", "ServiceAccount")

		role := &rbacv1.Role{
			ObjectMeta: meta("view-config", ns, 370-i*5),
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "list", "watch"},
			}},
		}
		b.addProto(kubePrefix+"/roles/"+ns+"/view-config", role, "rbac.authorization.k8s.io/v1", "Role")

		rb := &rbacv1.RoleBinding{
			ObjectMeta: meta("view-config", ns, 370-i*5),
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "view-config"},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "builder", Namespace: ns}},
		}
		b.addProto(kubePrefix+"/rolebindings/"+ns+"/view-config", rb, "rbac.authorization.k8s.io/v1", "RoleBinding")
	}

	cr := &rbacv1.ClusterRole{
		ObjectMeta: meta("demo-reader", "", 360),
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{""}, Resources: []string{"pods", "services"}, Verbs: []string{"get", "list"},
		}},
	}
	b.addProto(kubePrefix+"/clusterroles/demo-reader", cr, "rbac.authorization.k8s.io/v1", "ClusterRole")

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: meta("demo-reader", "", 360),
		RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "demo-reader"},
		Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "builder", Namespace: "demo-shop"}},
	}
	b.addProto(kubePrefix+"/clusterrolebindings/demo-reader", crb, "rbac.authorization.k8s.io/v1", "ClusterRoleBinding")
}

// etcdConfigMaps mirrors the static pod configuration an OpenShift control
// plane keeps, including the etcd-pod ConfigMap an operator looks for first.
func (b *builder) etcdConfigMaps() {
	for _, rev := range []string{"", "-56", "-57", "-58"} {
		name := "etcd-pod" + rev
		cm := &corev1.ConfigMap{
			ObjectMeta: meta(name, "openshift-etcd", 90),
			Data: map[string]string{
				"pod.yaml":                staticPodYAML,
				"version":                 "58",
				"forceRedeploymentReason": "single-master-recovery-2026-01-04",
			},
		}
		b.addProto(kubePrefix+"/configmaps/openshift-etcd/"+name, cm, "v1", "ConfigMap")
	}

	secret := &corev1.Secret{
		ObjectMeta: meta("etcd-client", "openshift-etcd", 200),
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nZGVtbyBjZXJ0aWZpY2F0ZQ==\n-----END CERTIFICATE-----\n"),
			"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nZGVtbyBrZXk=\n-----END PRIVATE KEY-----\n"),
		},
	}
	b.addProto(kubePrefix+"/secrets/openshift-etcd/etcd-client", secret, "v1", "Secret")
}

// etcdStaticPods adds the control plane pods a real OpenShift cluster runs in
// openshift-etcd. They carry no owner reference, because the kubelet creates
// them from a manifest on disk rather than a controller creating them.
func (b *builder) etcdStaticPods() {
	for i := 0; i < 3; i++ {
		node := fmt.Sprintf("master-%d.demo.example.com", i)
		for _, prefix := range []string{"etcd", "etcd-guard"} {
			name := prefix + "-" + node
			pod := &corev1.Pod{
				ObjectMeta: meta(name, "openshift-etcd", 90-i),
				Spec:       podSpec("etcd", 2379),
				Status: corev1.PodStatus{
					Phase:  corev1.PodRunning,
					PodIP:  fmt.Sprintf("192.0.2.%d", 10+i),
					HostIP: fmt.Sprintf("192.0.2.%d", 10+i),
					Conditions: []corev1.PodCondition{
						{Type: corev1.PodReady, Status: corev1.ConditionTrue},
					},
				},
			}
			pod.Labels = map[string]string{"app": "etcd", "k8s-app": "etcd"}
			pod.Spec.NodeName = node
			pod.Spec.HostNetwork = true
			b.addProto(fmt.Sprintf("%s/pods/openshift-etcd/%s", kubePrefix, name), pod, "v1", "Pod")
		}
	}
}

// workload builds a Deployment with its ReplicaSet, Pods, Service, Endpoints,
// Route and ConfigMap, wired together with owner references the way a real
// controller would.
func (b *builder) workload(seed int, ns, app string, replicas, port int32) {
	labels := map[string]string{"app": app}
	age := 200 - seed*13

	dep := &appsv1.Deployment{
		ObjectMeta: meta(app, ns, age),
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       podSpec(app, port),
			},
		},
		Status: appsv1.DeploymentStatus{Replicas: replicas, ReadyReplicas: replicas, AvailableReplicas: replicas},
	}
	dep.Labels = labels
	b.addProto(fmt.Sprintf("%s/deployments/%s/%s", kubePrefix, ns, app), dep, "apps/v1", "Deployment")

	rsName := fmt.Sprintf("%s-%x", app, 0x5d7c9f6b+seed)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: meta(rsName, ns, age),
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: podSpec(app, port)},
		},
		Status: appsv1.ReplicaSetStatus{Replicas: replicas, ReadyReplicas: replicas},
	}
	rs.Labels = labels
	rs.OwnerReferences = []metav1.OwnerReference{ownerRef("apps/v1", "Deployment", app, string(dep.UID))}
	b.addProto(fmt.Sprintf("%s/replicasets/%s/%s", kubePrefix, ns, rsName), rs, "apps/v1", "ReplicaSet")

	for i := int32(0); i < replicas; i++ {
		podName := fmt.Sprintf("%s-%s", rsName, podSuffix(seed, int(i)))
		pod := &corev1.Pod{
			ObjectMeta: meta(podName, ns, age-1),
			Spec:       podSpec(app, port),
			Status: corev1.PodStatus{
				Phase:  corev1.PodRunning,
				PodIP:  fmt.Sprintf("10.128.%d.%d", seed, 20+i),
				HostIP: "192.0.2.10",
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
					{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
				},
				ContainerStatuses: []corev1.ContainerStatus{{
					Name: app, Ready: true, RestartCount: i,
					Image: "registry.example.com/demo/" + app + ":1.4.2",
				}},
			},
		}
		pod.Labels = labels
		pod.Spec.NodeName = fmt.Sprintf("worker-%d.demo.example.com", int(i)%3)
		pod.OwnerReferences = []metav1.OwnerReference{ownerRef("apps/v1", "ReplicaSet", rsName, string(rs.UID))}
		b.addProto(fmt.Sprintf("%s/pods/%s/%s", kubePrefix, ns, podName), pod, "v1", "Pod")
	}

	svc := &corev1.Service{
		ObjectMeta: meta(app, ns, age),
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			Selector:  labels,
			ClusterIP: fmt.Sprintf("172.30.%d.%d", seed, 100+seed),
			Ports: []corev1.ServicePort{{
				Name: "http", Port: port, Protocol: corev1.ProtocolTCP, TargetPort: intstr.FromInt32(port),
			}},
		},
	}
	svc.Labels = labels
	b.addProto(fmt.Sprintf("%s/services/specs/%s/%s", kubePrefix, ns, app), svc, "v1", "Service")

	ep := &corev1.Endpoints{
		ObjectMeta: meta(app, ns, age),
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{IP: fmt.Sprintf("10.128.%d.20", seed)}},
			Ports:     []corev1.EndpointPort{{Name: "http", Port: port, Protocol: corev1.ProtocolTCP}},
		}},
	}
	b.addProto(fmt.Sprintf("%s/services/endpoints/%s/%s", kubePrefix, ns, app), ep, "v1", "Endpoints")

	cm := &corev1.ConfigMap{
		ObjectMeta: meta(app+"-config", ns, age),
		Data: map[string]string{
			"LOG_LEVEL":    "info",
			"UPSTREAM_URL": fmt.Sprintf("http://%s.%s.svc:%d", app, ns, port),
		},
	}
	b.addProto(fmt.Sprintf("%s/configmaps/%s/%s-config", kubePrefix, ns, app), cm, "v1", "ConfigMap")

	route := &routev1.Route{
		ObjectMeta: meta(app, ns, age),
		Spec: routev1.RouteSpec{
			Host: fmt.Sprintf("%s-%s.apps.demo.example.com", app, ns),
			To:   routev1.RouteTargetReference{Kind: "Service", Name: app},
			Port: &routev1.RoutePort{TargetPort: intstr.FromString("http")},
		},
	}
	b.addProto(fmt.Sprintf("%s/routes/%s/%s", openshiftPrefix, ns, app), route, "route.openshift.io/v1", "Route")
}

func podSpec(app string, port int32) corev1.PodSpec {
	return corev1.PodSpec{
		ServiceAccountName: "builder",
		Containers: []corev1.Container{{
			Name:            app,
			Image:           "registry.example.com/demo/" + app + ":1.4.2",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports:           []corev1.ContainerPort{{Name: "http", ContainerPort: port, Protocol: corev1.ProtocolTCP}},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			Env: []corev1.EnvVar{{Name: "APP_NAME", Value: app}},
		}},
	}
}

func ownerRef(apiVersion, kind, name, uid string) metav1.OwnerReference {
	controller := true
	return metav1.OwnerReference{
		APIVersion: apiVersion, Kind: kind, Name: name,
		UID: apitypesUID(uid), Controller: &controller,
	}
}

func podSuffix(seed, i int) string {
	const alphabet = "bcdfghjklmnpqrstvwxz2456789"
	out := make([]byte, 5)
	h := seed*131 + i*17 + 7
	for j := range out {
		out[j] = alphabet[h%len(alphabet)]
		h = h*31 + 11
	}
	return string(out)
}

// customResources are stored as JSON, the way the API server keeps anything
// served by a CustomResourceDefinition.
func (b *builder) customResources() {
	for i, app := range []string{"storefront", "checkout", "ingest", "platform"} {
		name := app + "-app"
		b.addJSON(fmt.Sprintf("%s/argoproj.io/applications/openshift-gitops/%s", kubePrefix, name), map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name":              name,
				"namespace":         "openshift-gitops",
				"uid":               string(types(name, "openshift-gitops")),
				"creationTimestamp": nsTime(150 - i*10).UTC().Format(time.RFC3339),
				"resourceVersion":   fmt.Sprintf("%d", 2000000+i*137),
				"finalizers":        []string{"resources-finalizer.argocd.argoproj.io"},
			},
			"spec": map[string]any{
				"project": "default",
				"source": map[string]any{
					"repoURL":        "https://git.example.com/demo/" + app + "-config.git",
					"targetRevision": "main",
					"path":           "chart",
				},
				"destination": map[string]any{"server": "https://kubernetes.default.svc", "namespace": "demo-shop"},
			},
			"status": map[string]any{
				"health": map[string]any{"status": []string{"Healthy", "Healthy", "Degraded", "Healthy"}[i]},
				"sync":   map[string]any{"status": []string{"Synced", "Synced", "OutOfSync", "Synced"}[i]},
			},
		})
	}

	b.addJSON(kubePrefix+"/apiextensions.k8s.io/customresourcedefinitions/applications.argoproj.io", map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]any{
			"name":              "applications.argoproj.io",
			"uid":               string(types("applications.argoproj.io", "")),
			"creationTimestamp": nsTime(300).UTC().Format(time.RFC3339),
		},
		"spec": map[string]any{
			"group": "argoproj.io",
			"names": map[string]any{"kind": "Application", "plural": "applications", "singular": "application"},
			"scope": "Namespaced",
			"versions": []any{map[string]any{
				"name": "v1alpha1", "served": true, "storage": true,
				"schema": map[string]any{"openAPIV3Schema": map[string]any{"type": "object"}},
			}},
		},
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": "Established", "status": "True"}},
		},
	})
}
