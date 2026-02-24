package probes

import (
	"context"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// probeTTL is the maximum age of a probe pod before it's considered orphaned.
	probeTTL = 5 * time.Minute
	// cleanupInterval is how often the cleanup loop runs.
	cleanupInterval = 60 * time.Second
)

// cleanupLoop periodically removes orphaned probe pods.
func (m *Manager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanupOrphans(ctx)
		}
	}
}

// cleanupOrphans deletes probe pods that have exceeded their TTL.
func (m *Manager) cleanupOrphans(ctx context.Context) {
	ns := m.cfg.ProbeNamespace

	pods, err := m.clients.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: LabelManagedBy + "=" + LabelManagedByValue,
	})
	if err != nil {
		slog.Debug("probe: cleanup failed to list pods", "namespace", ns, "error", err)
		return
	}

	now := time.Now()
	cleaned := 0

	for _, pod := range pods.Items {
		createdAtStr, ok := pod.Annotations[AnnotationCreatedAt]
		if !ok {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			continue
		}
		if now.Sub(createdAt) > probeTTL {
			if err := m.clients.Clientset.CoreV1().Pods(ns).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
				slog.Warn("probe: cleanup failed to delete pod", "pod", pod.Name, "error", err)
				continue
			}
			cleaned++
		}
	}

	if cleaned > 0 {
		slog.Info("probe: cleaned up orphaned pods", "count", cleaned, "namespace", ns)
	}
}
