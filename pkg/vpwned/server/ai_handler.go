package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	migrationv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

const (
	defaultAIURL       = "http://vjailbreak-ai.migration-system.svc.cluster.local:8080"
	debugLogsBaseURL   = "http://vjailbreak-ui-service.migration-system.svc.cluster.local/debug-logs"
	podLogContextLines = 10
	podLogTailLines    = 200
	controllerNS       = "migration-system"
	controllerLabel    = "control-plane=controller-manager"
	maxDebugLogFiles   = 10
)

type aiAnalyzeRequest struct {
	MigrationName       string              `json:"migration_name"`
	Namespace           string              `json:"namespace"`
	Question            string              `json:"question,omitempty"`
	ConversationHistory []map[string]string `json:"conversation_history"`
}

type fetchContextFn func(migrationName, namespace string) (map[string]any, error)

type aiAnalyzeHandler struct {
	k8sClient    client.Client
	rawK8s       kubernetes.Interface
	httpClient   *http.Client
	aiURL        string
	fetchContext fetchContextFn
}

func NewAIAnalyzeHandler(k8sClient client.Client, rawK8s kubernetes.Interface) *aiAnalyzeHandler {
	h := &aiAnalyzeHandler{
		k8sClient:  k8sClient,
		rawK8s:     rawK8s,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		aiURL:      getEnvOrDefault("VJAILBREAK_AI_URL", defaultAIURL),
	}
	h.fetchContext = h.assembleMigrationContext
	return h
}

func (h *aiAnalyzeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req aiAnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.MigrationName == "" || req.Namespace == "" {
		http.Error(w, "migration_name and namespace are required", http.StatusBadRequest)
		return
	}

	migCtx, err := h.fetchContext(req.MigrationName, req.Namespace)
	if err != nil {
		logrus.Errorf("ai_handler: failed to assemble context for %s: %v", req.MigrationName, err)
		http.Error(w, "failed to collect migration data", http.StatusInternalServerError)
		return
	}

	payload := map[string]any{
		"migration_name":       req.MigrationName,
		"namespace":            req.Namespace,
		"context":              migCtx,
		"conversation_history": req.ConversationHistory,
		"question":             req.Question,
	}

	payloadBytes, _ := json.Marshal(payload)
	logrus.Debugf("ai_handler: sending payload to vjailbreak-ai: %s", string(payloadBytes))
	aiResp, err := h.httpClient.Post(
		h.aiURL+"/analyze-migration",
		"application/json",
		bytes.NewReader(payloadBytes),
	)
	if err != nil {
		logrus.Errorf("ai_handler: vjailbreak-ai call failed: %v", err)
		http.Error(w, "AI service unavailable", http.StatusBadGateway)
		return
	}
	defer aiResp.Body.Close()

	respBody, _ := io.ReadAll(aiResp.Body)
	if aiResp.StatusCode != http.StatusOK {
		logrus.Errorf("ai_handler: vjailbreak-ai returned %d: %s", aiResp.StatusCode, string(respBody))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(aiResp.StatusCode)
	w.Write(respBody) //nolint:errcheck
}

func (h *aiAnalyzeHandler) assembleMigrationContext(migrationName, namespace string) (map[string]any, error) {
	ctx := context.Background()

	var migration migrationv1alpha1.Migration
	if err := h.k8sClient.Get(ctx, types.NamespacedName{Name: migrationName, Namespace: namespace}, &migration); err != nil {
		return nil, fmt.Errorf("get migration CR: %w", err)
	}

	migrationCR := map[string]any{
		"metadata": map[string]any{
			"name":      migration.Name,
			"namespace": migration.Namespace,
		},
		"spec":   migration.Spec,
		"status": migration.Status,
	}

	var migrationPlan any
	var migrationTemplate any
	var networkMapping any
	var storageMapping any

	if planName := migration.Spec.MigrationPlan; planName != "" {
		var plan migrationv1alpha1.MigrationPlan
		if err := h.k8sClient.Get(ctx, types.NamespacedName{Name: planName, Namespace: namespace}, &plan); err == nil {
			migrationPlan = plan.Spec

			var tmpl migrationv1alpha1.MigrationTemplate
			if err2 := h.k8sClient.Get(ctx, types.NamespacedName{Name: plan.Spec.MigrationTemplate, Namespace: namespace}, &tmpl); err2 == nil {
				// Strip credential references (Source.VMwareRef, Destination.OpenstackRef)
				tmplSpec := map[string]any{
					"networkMapping":    tmpl.Spec.NetworkMapping,
					"storageMapping":    tmpl.Spec.StorageMapping,
					"osFamily":          tmpl.Spec.OSFamily,
					"storageCopyMethod": tmpl.Spec.StorageCopyMethod,
				}
				migrationTemplate = tmplSpec

				if nmName := tmpl.Spec.NetworkMapping; nmName != "" {
					var nm migrationv1alpha1.NetworkMapping
					if err3 := h.k8sClient.Get(ctx, types.NamespacedName{Name: nmName, Namespace: namespace}, &nm); err3 == nil {
						networkMapping = nm.Spec
					}
				}

				if smName := tmpl.Spec.StorageMapping; smName != "" {
					var sm migrationv1alpha1.StorageMapping
					if err3 := h.k8sClient.Get(ctx, types.NamespacedName{Name: smName, Namespace: namespace}, &sm); err3 == nil {
						storageMapping = sm.Spec
					}
				}
			}
		}
	}

	fetchWarnings := []string{}

	v2vLogs := ""
	if podName := migration.Spec.PodRef; podName != "" && h.rawK8s != nil {
		lines, err := h.fetchPodLogs(ctx, namespace, podName)
		if err != nil {
			logrus.Warnf("ai_handler: failed to fetch v2v pod logs for %s: %v", podName, err)
			fetchWarnings = append(fetchWarnings, fmt.Sprintf("v2v-helper pod logs unavailable: %v", err))
		} else {
			extracted := ExtractRelevantLines(lines, podLogContextLines, podLogTailLines)
			v2vLogs = strings.Join(extracted, "\n")
		}
	}

	controllerLogs := ""
	if h.rawK8s != nil {
		if podName, err := h.findControllerPod(ctx); err == nil {
			lines, err := h.fetchPodLogs(ctx, controllerNS, podName)
			if err != nil {
				logrus.Warnf("ai_handler: failed to fetch controller logs: %v", err)
				fetchWarnings = append(fetchWarnings, fmt.Sprintf("controller pod logs unavailable: %v", err))
			} else {
				extracted := ExtractRelevantLines(lines, podLogContextLines, 0)
				controllerLogs = strings.Join(extracted, "\n")
			}
		}
	}

	debugLogs, debugWarns, err := h.fetchDebugLogs(migrationName)
	fetchWarnings = append(fetchWarnings, debugWarns...)
	if err != nil {
		logrus.Warnf("ai_handler: failed to fetch debug logs for %s: %v", migrationName, err)
		if debugLogs == nil {
			debugLogs = map[string]string{}
		}
	}

	additionalContext := ""
	var ctxCM corev1.ConfigMap
	if err := h.k8sClient.Get(ctx, types.NamespacedName{Name: "vjailbreak-ai-context", Namespace: controllerNS}, &ctxCM); err == nil {
		additionalContext = ctxCM.Data["additional_context"]
	}

	return map[string]any{
		"migration_cr":       migrationCR,
		"migration_plan":     nilToEmptyMap(migrationPlan),
		"migration_template": nilToEmptyMap(migrationTemplate),
		"network_mapping":    nilToEmptyMap(networkMapping),
		"storage_mapping":    nilToEmptyMap(storageMapping),
		"v2v_logs":           v2vLogs,
		"controller_logs":    controllerLogs,
		"debug_logs":         debugLogs,
		"additional_context": additionalContext,
		"fetch_warnings":     fetchWarnings,
	}, nil
}

func (h *aiAnalyzeHandler) fetchPodLogs(ctx context.Context, namespace, podName string) ([]string, error) {
	req := h.rawK8s.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	rc, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}
	return SplitLines(string(data)), nil
}

func (h *aiAnalyzeHandler) findControllerPod(ctx context.Context) (string, error) {
	pods, err := h.rawK8s.CoreV1().Pods(controllerNS).List(ctx, metav1.ListOptions{
		LabelSelector: controllerLabel,
	})
	if err != nil || len(pods.Items) == 0 {
		return "", fmt.Errorf("controller pod not found")
	}
	return pods.Items[0].Name, nil
}

// fetchDebugLogs fetches up to maxDebugLogFiles migration log files from the nginx /debug-logs/ endpoint.
// Returns (logs map, warnings, error). Warnings are non-fatal partial failures.
func (h *aiAnalyzeHandler) fetchDebugLogs(migrationName string) (map[string]string, []string, error) {
	result := map[string]string{}
	var warnings []string

	listResp, err := h.httpClient.Get(debugLogsBaseURL + "/")
	if err != nil {
		return result, warnings, fmt.Errorf("listing debug logs: %w", err)
	}
	defer listResp.Body.Close()

	var entries []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		return result, warnings, fmt.Errorf("decoding debug log listing: %w", err)
	}

	for _, entry := range entries {
		if !strings.Contains(entry.Name, migrationName) {
			continue
		}
		if entry.Type == "directory" {
			subResp, err := h.httpClient.Get(debugLogsBaseURL + "/" + entry.Name + "/")
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("debug logs subdir %s unavailable: %v", entry.Name, err))
				continue
			}
			var subEntries []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			}
			json.NewDecoder(subResp.Body).Decode(&subEntries) //nolint:errcheck
			subResp.Body.Close()

			for _, sub := range subEntries {
				if len(result) >= maxDebugLogFiles {
					break
				}
				if !strings.HasSuffix(sub.Name, ".log") {
					continue
				}
				content, err := h.fetchFileContent(debugLogsBaseURL + "/" + entry.Name + "/" + sub.Name)
				if err == nil {
					extracted := ExtractRelevantLines(SplitLines(content), podLogContextLines, podLogTailLines)
					result[entry.Name+"/"+sub.Name] = strings.Join(extracted, "\n")
				}
			}
		} else if strings.HasSuffix(entry.Name, ".log") {
			if len(result) < maxDebugLogFiles {
				content, err := h.fetchFileContent(debugLogsBaseURL + "/" + entry.Name)
				if err == nil {
					extracted := ExtractRelevantLines(SplitLines(content), podLogContextLines, podLogTailLines)
					result[entry.Name] = strings.Join(extracted, "\n")
				}
			}
		}
	}
	return result, warnings, nil
}

func (h *aiAnalyzeHandler) fetchFileContent(url string) (string, error) {
	resp, err := h.httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return string(data), err
}

func nilToEmptyMap(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}

func getEnvOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
