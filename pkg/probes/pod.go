package probes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/isitobservable/k8s-networking-mcp/pkg/config"
	"github.com/isitobservable/k8s-networking-mcp/pkg/k8s"
)

// podCounter provides unique pod names across concurrent probes.
var podCounter atomic.Int64

// createProbePod creates an ephemeral pod in the given namespace with the probe command.
func createProbePod(ctx context.Context, clients *k8s.Clients, cfg *config.Config, namespace string, req ProbeRequest) (string, error) {
	podName := fmt.Sprintf("mcp-probe-%s-%d-%d", req.Type, time.Now().Unix(), podCounter.Add(1))

	falseVal := false
	trueVal := true
	var runAsUser int64 = 1000

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				LabelManagedBy: LabelManagedByValue,
				LabelProbeType: string(req.Type),
			},
			Annotations: map[string]string{
				AnnotationCreatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "probe",
					Image:   cfg.ProbeImage,
					Command: req.Command,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("32Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsNonRoot:             &trueVal,
						RunAsUser:                &runAsUser,
						AllowPrivilegeEscalation: &falseVal,
						ReadOnlyRootFilesystem:   &trueVal,
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
				},
			},
		},
	}

	created, err := clients.Clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	slog.Debug("probe: created pod", "pod", created.Name, "namespace", namespace, "type", req.Type)
	return created.Name, nil
}

// deleteProbePod removes the probe pod.
func deleteProbePod(ctx context.Context, clients *k8s.Clients, namespace, podName string) error {
	return clients.Clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// waitForPod watches the pod until it reaches a terminal state and collects logs.
func waitForPod(ctx context.Context, clients *k8s.Clients, namespace, podName string) (*ProbeResult, error) {
	watcher, err := clients.Clientset.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch pod %s: %w", podName, err)
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return nil, fmt.Errorf("pod watch channel closed")
			}
			if event.Type == watch.Deleted {
				return &ProbeResult{Success: false, Error: "pod was deleted unexpectedly"}, nil
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				output := collectLogs(ctx, clients, namespace, podName)
				return &ProbeResult{
					Success:  true,
					Output:   output,
					ExitCode: 0,
				}, nil
			case corev1.PodFailed:
				output := collectLogs(ctx, clients, namespace, podName)
				exitCode := 1
				if len(pod.Status.ContainerStatuses) > 0 {
					if terminated := pod.Status.ContainerStatuses[0].State.Terminated; terminated != nil {
						exitCode = int(terminated.ExitCode)
					}
				}
				return &ProbeResult{
					Success:  false,
					Output:   output,
					ExitCode: exitCode,
					Error:    "probe command failed",
				}, nil
			}
		}
	}
}

// collectLogs retrieves the logs from the probe container.
func collectLogs(ctx context.Context, clients *k8s.Clients, namespace, podName string) string {
	logReq := clients.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "probe",
	})
	stream, err := logReq.Stream(ctx)
	if err != nil {
		slog.Warn("probe: failed to get logs", "pod", podName, "error", err)
		return ""
	}
	defer func() { _ = stream.Close() }()

	var buf bytes.Buffer
	// Limit log output to 64KB
	if _, err := io.Copy(&buf, io.LimitReader(stream, 64*1024)); err != nil {
		slog.Warn("probe: error reading logs", "pod", podName, "error", err)
	}
	return buf.String()
}
