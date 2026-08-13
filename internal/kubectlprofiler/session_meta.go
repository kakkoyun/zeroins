package kubectlprofiler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/kakkoyun/zeroins/internal/sessions"
)

type profilerSessionMeta struct {
	ID         string
	AttachedAt string
	ExpiresAt  string
	Endpoint   string
}

func newProfilerSessionMeta(endpoint string, duration time.Duration) profilerSessionMeta {
	meta := profilerSessionMeta{
		ID:         generateProfilerSessionID(),
		AttachedAt: time.Now().UTC().Format(time.RFC3339),
		Endpoint:   endpoint,
	}
	if duration > 0 {
		meta.ExpiresAt = time.Now().Add(duration).UTC().Format(time.RFC3339)
	}
	return meta
}

func generateProfilerSessionID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("profiler-%s", hex.EncodeToString(b))
}

func labelAndAnnotateProfilerDaemonSet(ctx context.Context, deps Dependencies, dsName, namespace string, meta profilerSessionMeta) error {
	labelArgs := []string{"label", "daemonset", dsName, "-n", namespace, sessions.ManagedLabel + "=" + sessions.ManagedLabelValue, "--overwrite"}
	if _, err := deps.Runner.Run(ctx, "kubectl", labelArgs...); err != nil {
		return fmt.Errorf("label daemonset: %w", err)
	}
	annotations := map[string]string{
		sessions.SessionIDAnnotation:  meta.ID,
		sessions.AttachedAtAnnotation: meta.AttachedAt,
		sessions.EndpointAnnotation:   meta.Endpoint,
		sessions.ModeAnnotation:       "daemonset",
		sessions.ToolAnnotation:       "profiler",
	}
	if meta.ExpiresAt != "" {
		annotations[sessions.ExpiresAtAnnotation] = meta.ExpiresAt
	}
	annotateArgs := []string{"annotate", "daemonset", dsName, "-n", namespace, "--overwrite"}
	for key, value := range annotations {
		annotateArgs = append(annotateArgs, key+"="+value)
	}
	if _, err := deps.Runner.Run(ctx, "kubectl", annotateArgs...); err != nil {
		return fmt.Errorf("annotate daemonset: %w", err)
	}
	return nil
}
