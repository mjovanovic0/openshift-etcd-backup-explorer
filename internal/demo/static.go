package demo

import (
	"archive/tar"
	"compress/gzip"
	"os"

	apitypes "k8s.io/apimachinery/pkg/types"
)

// apitypesUID is an alias so the cluster builder does not need to import the
// Kubernetes types package directly.
type apitypesUID = apitypes.UID

// staticPodYAML is the manifest the etcd static pod ConfigMap carries. It is
// trimmed but keeps the shape an operator expects to find.
const staticPodYAML = `apiVersion: v1
kind: Pod
metadata:
  name: etcd
  namespace: openshift-etcd
  labels:
    app: etcd
    k8s-app: etcd
    revision: "58"
spec:
  hostNetwork: true
  containers:
    - name: etcd
      image: registry.example.com/openshift/etcd:4.18
      command:
        - /bin/sh
        - -c
        - exec etcd --logger=zap --config-file=/etc/kubernetes/etcd.conf
      ports:
        - containerPort: 2379
          name: etcd
          protocol: TCP
        - containerPort: 2380
          name: etcd-peer
          protocol: TCP
      resources:
        requests:
          cpu: 300m
          memory: 600Mi
`

// staticFiles is the layout OpenShift writes into the static resources
// tarball beside a snapshot.
var staticFiles = map[string]string{
	"static-pod-resources/etcd-pod-58/configmaps/etcd-pod/pod.yaml":                staticPodYAML,
	"static-pod-resources/etcd-pod-58/configmaps/etcd-pod/version":                 "58\n",
	"static-pod-resources/etcd-pod-58/configmaps/etcd-pod/forceRedeploymentReason": "single-master-recovery-2026-01-04\n",
	"static-pod-resources/etcd-pod-58/configmaps/etcd-serving-ca/ca-bundle.crt":    demoCert,
	"static-pod-resources/etcd-pod-58/secrets/etcd-all-certs/etcd-peer.crt":        demoCert,
	"static-pod-resources/etcd-pod-58/etcd-pod.yaml":                               staticPodYAML,
	"static-pod-resources/kube-apiserver-pod-12/configmaps/config/config.yaml":     "apiVersion: kubecontrolplane.config.openshift.io/v1\nkind: KubeAPIServerConfig\n",
	"static-pod-resources/kube-apiserver-pod-12/kube-apiserver-pod.yaml":           "apiVersion: v1\nkind: Pod\nmetadata:\n  name: kube-apiserver\n  namespace: openshift-kube-apiserver\n",
}

const demoCert = `-----BEGIN CERTIFICATE-----
ZGVtbyBjZXJ0aWZpY2F0ZSwgbm90IGEgcmVhbCBvbmU=
-----END CERTIFICATE-----
`

// writeStaticArchive produces the static_kuberesources tarball.
func writeStaticArchive(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// A stable order keeps the fixture reproducible.
	for _, name := range sortedKeys(staticFiles) {
		body := staticFiles[name]
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Mode:     0o600,
			Size:     int64(len(body)),
			ModTime:  TakenAt,
			Typeflag: tar.TypeReg,
		}); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			return err
		}
	}
	return nil
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
