package kubectlobi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/kakkoyun/zeroins/internal/sessions"
)

// sessionMeta holds the annotations and label to apply to a managed resource.
type sessionMeta struct {
	ID         string
	AttachedAt string
	ExpiresAt  string
	Endpoint   string
	Mode       string
}

func newSessionMeta(endpoint, mode string, duration time.Duration) sessionMeta {
	meta := sessionMeta{
		ID:         generateSessionID("obi"),
		AttachedAt: time.Now().UTC().Format(time.RFC3339),
		Endpoint:   endpoint,
		Mode:       mode,
	}
	if duration > 0 {
		meta.ExpiresAt = time.Now().Add(duration).UTC().Format(time.RFC3339)
	}
	return meta
}

func generateSessionID(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

// labelAndAnnotateDaemonSet applies the managed label and session annotations
// to the DaemonSet created by the Helm release.
func labelAndAnnotateDaemonSet(ctx context.Context, deps Dependencies, dsName, namespace string, meta sessionMeta) error {
	labelArgs := []string{"label", "daemonset", dsName, "-n", namespace, sessions.ManagedLabel + "=" + sessions.ManagedLabelValue, "--overwrite"}
	if _, err := deps.Runner.Run(ctx, "kubectl", labelArgs...); err != nil {
		return fmt.Errorf("label daemonset: %w", err)
	}
	annotations := map[string]string{
		sessions.SessionIDAnnotation:  meta.ID,
		sessions.AttachedAtAnnotation: meta.AttachedAt,
		sessions.EndpointAnnotation:   meta.Endpoint,
		sessions.ModeAnnotation:       meta.Mode,
		sessions.ToolAnnotation:       "obi",
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

// sidecarAnnotations returns the annotations to include in the sidecar patch.
func sidecarAnnotations(meta sessionMeta) map[string]string {
	ann := map[string]string{
		managedAnnotation:             "true",
		originalShareAnnotation:       "unset", // filled from live state at attach time
		sessions.SessionIDAnnotation:  meta.ID,
		sessions.AttachedAtAnnotation: meta.AttachedAt,
		sessions.EndpointAnnotation:   meta.Endpoint,
		sessions.ModeAnnotation:       meta.Mode,
		sessions.ToolAnnotation:       "obi",
	}
	if meta.ExpiresAt != "" {
		ann[sessions.ExpiresAtAnnotation] = meta.ExpiresAt
	}
	return ann
}
