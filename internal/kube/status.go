package kube

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Status is a short health summary for the resource list, plus a tone the UI
// turns into a colored dot.
type Status struct {
	Text string `json:"text"`
	Tone string `json:"tone"` // ok, warn, error, info, neutral
}

// Summarize derives a one word status from a decoded object. Kubernetes has no
// single status field, so this walks a small set of rules per kind and falls
// back to the standard condition list.
func Summarize(kind string, body []byte) Status {
	var o map[string]any
	if err := json.Unmarshal(body, &o); err != nil {
		return Status{}
	}
	status, _ := o["status"].(map[string]any)
	spec, _ := o["spec"].(map[string]any)
	meta, _ := o["metadata"].(map[string]any)

	if meta != nil {
		if _, deleting := meta["deletionTimestamp"]; deleting {
			return Status{Text: "Terminating", Tone: "warn"}
		}
	}

	switch kind {
	case "Pod":
		phase, _ := status["phase"].(string)
		return Status{Text: phase, Tone: phaseTone(phase)}

	case "Namespace", "Project", "PersistentVolume":
		phase, _ := status["phase"].(string)
		return Status{Text: phase, Tone: phaseTone(phase)}

	case "PersistentVolumeClaim":
		phase, _ := status["phase"].(string)
		return Status{Text: phase, Tone: phaseTone(phase)}

	case "Deployment", "StatefulSet", "ReplicaSet":
		ready := num(status, "readyReplicas")
		want := num(spec, "replicas")
		tone := "ok"
		if ready < want {
			tone = "warn"
		}
		if want > 0 && ready == 0 {
			tone = "error"
		}
		return Status{Text: fmt.Sprintf("%d/%d ready", ready, want), Tone: tone}

	case "DaemonSet":
		ready := num(status, "numberReady")
		want := num(status, "desiredNumberScheduled")
		tone := "ok"
		if ready < want {
			tone = "warn"
		}
		return Status{Text: fmt.Sprintf("%d/%d ready", ready, want), Tone: tone}

	case "Job":
		switch {
		case num(status, "succeeded") > 0:
			return Status{Text: "Complete", Tone: "ok"}
		case num(status, "failed") > 0:
			return Status{Text: "Failed", Tone: "error"}
		case num(status, "active") > 0:
			return Status{Text: "Active", Tone: "info"}
		}
		return Status{Text: "Pending", Tone: "neutral"}

	case "CronJob":
		if suspended, ok := spec["suspend"].(bool); ok && suspended {
			return Status{Text: "Suspended", Tone: "neutral"}
		}
		sched, _ := spec["schedule"].(string)
		return Status{Text: sched, Tone: "info"}

	case "Node":
		return conditionStatus(status, "Ready")

	case "Service":
		t, _ := spec["type"].(string)
		return Status{Text: t, Tone: "info"}

	case "Secret":
		t, _ := o["type"].(string)
		return Status{Text: shortSecretType(t), Tone: "neutral"}

	case "ConfigMap":
		d, _ := o["data"].(map[string]any)
		b, _ := o["binaryData"].(map[string]any)
		return Status{Text: plural(len(d)+len(b), "key"), Tone: "neutral"}

	case "Event":
		t, _ := o["type"].(string)
		tone := "info"
		if t == "Warning" {
			tone = "warn"
		}
		return Status{Text: t, Tone: tone}

	case "Route":
		h, _ := spec["host"].(string)
		return Status{Text: h, Tone: "info"}

	case "Application":
		// Argo CD, the most common custom resource in these clusters.
		health, _ := status["health"].(map[string]any)
		sync, _ := status["sync"].(map[string]any)
		hs, _ := health["status"].(string)
		ss, _ := sync["status"].(string)
		text := strings.TrimSpace(strings.Join([]string{hs, ss}, " / "))
		tone := "neutral"
		switch {
		case hs == "Healthy" && ss == "Synced":
			tone = "ok"
		case hs == "Degraded" || hs == "Missing":
			tone = "error"
		case hs != "" || ss != "":
			tone = "warn"
		}
		return Status{Text: strings.Trim(text, " /"), Tone: tone}
	}

	// Anything else: look for a conventional condition.
	for _, want := range []string{"Ready", "Available", "Established", "Succeeded", "Synced"} {
		if s := conditionStatus(status, want); s.Text != "" {
			return s
		}
	}
	if phase, ok := status["phase"].(string); ok {
		return Status{Text: phase, Tone: phaseTone(phase)}
	}
	return Status{}
}

func conditionStatus(status map[string]any, want string) Status {
	conds, _ := status["conditions"].([]any)
	for _, c := range conds {
		cm, _ := c.(map[string]any)
		if t, _ := cm["type"].(string); t != want {
			continue
		}
		v, _ := cm["status"].(string)
		switch v {
		case "True":
			return Status{Text: want, Tone: "ok"}
		case "False":
			return Status{Text: "Not" + want, Tone: "error"}
		default:
			return Status{Text: want + " unknown", Tone: "warn"}
		}
	}
	return Status{}
}

func phaseTone(phase string) string {
	switch phase {
	case "Running", "Active", "Bound", "Available", "Succeeded", "Complete":
		return "ok"
	case "Pending", "Released":
		return "warn"
	case "Failed", "Terminating", "Lost":
		return "error"
	case "":
		return ""
	}
	return "info"
}

func num(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case json.Number:
		i, _ := v.Int64()
		return i
	}
	return 0
}

// shortSecretType trims the well known kubernetes.io/ prefix so the column
// stays narrow.
func shortSecretType(t string) string {
	return strings.TrimPrefix(t, "kubernetes.io/")
}

// plural renders a count with a correctly pluralized noun.
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
