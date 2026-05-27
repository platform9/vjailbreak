package timesettings

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Host paths reachable from the container via hostPath volume mounts.
const (
	TimesyncdConfDir  = "/etc/systemd/timesyncd.conf.d"
	TimesyncdConfFile = "/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf"
	ZoneinfoBase      = "/usr/share/zoneinfo"
	Pf9EnvPath        = "/etc/pf9/env"
)

var (
	timesyncdConfDirOverride  = TimesyncdConfDir
	timesyncdConfFileOverride = TimesyncdConfFile
)

// WorkloadKind identifies which controller type owns a pod that needs to be
// restarted to pick up TZ env changes.
type WorkloadKind int

const (
	WorkloadDeployment WorkloadKind = iota
	WorkloadStatefulSet
)

// WorkloadRef points at a Deployment or StatefulSet that consumes the TZ
// environment variable and must be restarted when timezone changes.
type WorkloadRef struct {
	Kind      WorkloadKind
	Name      string
	Namespace string
}

var workloadsToRestart = []WorkloadRef{
	{WorkloadDeployment, "migration-controller-manager", constants.NamespaceMigrationSystem},
	{WorkloadDeployment, "vjailbreak-ui", constants.NamespaceMigrationSystem},
	{WorkloadDeployment, "grafana", "monitoring"},
	{WorkloadStatefulSet, "prometheus-k8s", "monitoring"},
}

var (
	ipv4RE     = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	hostnameRE = regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	labelRE    = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
)

// IsValidNTPServer returns true if s is a syntactically valid IPv4 address or
// hostname suitable for use as an NTP server.
func IsValidNTPServer(s string) bool {
	if s == "" || strings.Contains(s, "://") || strings.Contains(s, "/") {
		return false
	}
	if ipv4RE.MatchString(s) {
		for _, part := range strings.Split(s, ".") {
			v := 0
			fmt.Sscanf(part, "%d", &v)
			if v < 0 || v > 255 {
				return false
			}
		}
		return true
	}
	if !hostnameRE.MatchString(s) || strings.HasPrefix(s, ".") || strings.Contains(s, "..") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || !labelRE.MatchString(label) {
			return false
		}
	}
	return true
}

// FilterValidNTPServers splits a comma/newline/space-separated list, drops any
// invalid entries (with a warning log), and returns the survivors as a single
// space-separated string suitable for systemd-timesyncd's NTP= directive.
func FilterValidNTPServers(raw string) string {
	raw = strings.NewReplacer(",", " ", "\n", " ").Replace(raw)
	var valid, invalid []string
	for _, s := range strings.Fields(raw) {
		if IsValidNTPServer(s) {
			valid = append(valid, s)
		} else {
			invalid = append(invalid, s)
		}
	}
	if len(invalid) > 0 {
		logrus.Warnf("timesettings: ignoring invalid NTP server entries: %v", invalid)
	}
	return strings.Join(valid, " ")
}

// writeTimesyncdConf writes (or removes) the 99-vjailbreak.conf override that
// configures NTP_SERVERS for systemd-timesyncd.
//
// This is the SYNC-layer change. After this returns, systemd-timesyncd needs
// to be restarted via D-Bus to actually re-read the file.
func writeTimesyncdConf(servers string) error {
	if err := os.MkdirAll(timesyncdConfDirOverride, 0755); err != nil {
		return fmt.Errorf("create timesyncd conf dir: %w", err)
	}
	if servers == "" {
		if err := os.Remove(timesyncdConfFileOverride); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove timesyncd conf: %w", err)
		}
		return nil
	}
	return os.WriteFile(timesyncdConfFileOverride, []byte(fmt.Sprintf("[Time]\nNTP=%s\n", servers)), 0644)
}

func sanitizeTimezone(tz string) (string, error) {
	if tz == "" {
		return "", nil
	}
	if strings.Contains(tz, "..") || strings.HasPrefix(tz, "/") || strings.ContainsRune(tz, 0) {
		return "", fmt.Errorf("timezone %q contains invalid characters", tz)
	}
	target := filepath.Join(ZoneinfoBase, tz)
	rel, err := filepath.Rel(ZoneinfoBase, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("timezone %q resolves outside zoneinfo directory", tz)
	}
	return tz, nil
}

// notifyTimedateViaDbus calls org.freedesktop.timedate1 over the host's system
// bus so timedated updates host timezone/NTP state. Failures here are returned
// as warnings by Apply so the caller can distinguish hard config failures from
// host reconciliation issues.
func notifyTimedateViaDbus(tz string, ntpEnabled bool) error {
	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return fmt.Errorf("connect to system D-Bus: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return fmt.Errorf("D-Bus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		return fmt.Errorf("D-Bus hello: %w", err)
	}

	obj := conn.Object("org.freedesktop.timedate1", "/org/freedesktop/timedate1")
	var errs []error
	if tz != "" {
		if call := obj.Call("org.freedesktop.timedate1.SetTimezone", 0, tz, false); call.Err != nil {
			errs = append(errs, fmt.Errorf("D-Bus SetTimezone: %w", call.Err))
		}
	}
	if call := obj.Call("org.freedesktop.timedate1.SetNTP", 0, ntpEnabled, false); call.Err != nil {
		errs = append(errs, fmt.Errorf("D-Bus SetNTP: %w", call.Err))
	}
	return errors.Join(errs...)
}

// restartTimesyncdViaDbus restarts systemd-timesyncd via the systemd manager
// D-Bus interface so it re-reads the timesyncd.conf.d/ override.
func restartTimesyncdViaDbus() error {
	conn, err := dbus.SystemBusPrivate()
	if err != nil {
		return fmt.Errorf("connect to system D-Bus: %w", err)
	}
	defer conn.Close()
	if err := conn.Auth(nil); err != nil {
		return fmt.Errorf("D-Bus auth: %w", err)
	}
	if err := conn.Hello(); err != nil {
		return fmt.Errorf("D-Bus hello: %w", err)
	}

	obj := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	var jobPath dbus.ObjectPath
	call := obj.Call("org.freedesktop.systemd1.Manager.RestartUnit", 0, "systemd-timesyncd.service", "replace")
	if call.Err != nil {
		return fmt.Errorf("D-Bus RestartUnit timesyncd: %w", call.Err)
	}
	_ = call.Store(&jobPath)
	return nil
}

// updatePf9EnvFile writes/replaces the TZ= line in /etc/pf9/env so that future
// container starts that source it pick up the new timezone.
func updatePf9EnvFile(tz string) error {
	if tz == "" {
		tz = "UTC"
	}
	data, err := os.ReadFile(Pf9EnvPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", Pf9EnvPath, err)
	}
	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "TZ=") {
			lines[i] = "TZ=" + tz
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, "TZ="+tz)
	}
	if err := os.WriteFile(Pf9EnvPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return fmt.Errorf("write %s: %w", Pf9EnvPath, err)
	}
	return nil
}

// patchPf9EnvConfigMap sets data.TZ on the pf9-env ConfigMap so newly-launched
// pods that mount it as envFrom get the correct TZ.
func patchPf9EnvConfigMap(ctx context.Context, k8sClient client.Client, tz string) error {
	if tz == "" {
		tz = "UTC"
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm := &corev1.ConfigMap{}
		if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
			Name:      "pf9-env",
			Namespace: constants.NamespaceMigrationSystem,
		}, cm); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get pf9-env configmap: %w", err)
		}
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		if cm.Data["TZ"] == tz {
			return nil
		}
		cm.Data["TZ"] = tz
		return k8sClient.Update(ctx, cm)
	})
}

// restartTZWorkloads triggers a rollout restart on every Deployment and
// StatefulSet that consumes the TZ env var. Errors are aggregated so one
// missing workload doesn't prevent the others from being restarted.
func restartTZWorkloads(ctx context.Context, k8sClient client.Client) error {
	now := time.Now().Format(time.RFC3339)
	var errs []error
	for _, w := range workloadsToRestart {
		w := w
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var obj client.Object
			switch w.Kind {
			case WorkloadDeployment:
				obj = &appsv1.Deployment{}
			case WorkloadStatefulSet:
				obj = &appsv1.StatefulSet{}
			default:
				return fmt.Errorf("unknown workload kind for %s/%s", w.Namespace, w.Name)
			}
			if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
				Name:      w.Name,
				Namespace: w.Namespace,
			}, obj); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			switch o := obj.(type) {
			case *appsv1.Deployment:
				if o.Spec.Template.Annotations == nil {
					o.Spec.Template.Annotations = make(map[string]string)
				}
				o.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = now
			case *appsv1.StatefulSet:
				if o.Spec.Template.Annotations == nil {
					o.Spec.Template.Annotations = make(map[string]string)
				}
				o.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = now
			}
			return k8sClient.Update(ctx, obj)
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("restart %s/%s: %w", w.Namespace, w.Name, err))
		}
	}
	return errors.Join(errs...)
}

func RestartDeployment(ctx context.Context, k8sClient client.Client, name, namespace string) error {
	now := time.Now().Format(time.RFC3339)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj := &appsv1.Deployment{}
		if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
			Name:      name,
			Namespace: namespace,
		}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if obj.Spec.Template.Annotations == nil {
			obj.Spec.Template.Annotations = make(map[string]string)
		}
		obj.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = now
		return k8sClient.Update(ctx, obj)
	})
}

// patchVersionCheckerTZ sets spec.timeZone on the version-checker CronJob so
// the cron schedule fires in the configured timezone.
func patchVersionCheckerTZ(ctx context.Context, k8sClient client.Client, tz string) error {
	if tz == "" {
		tz = "UTC"
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cj := &batchv1.CronJob{}
		if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
			Name:      "vjailbreak-version-checker",
			Namespace: constants.NamespaceMigrationSystem,
		}, cj); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("get version-checker cronjob: %w", err)
		}
		if cj.Spec.TimeZone != nil && *cj.Spec.TimeZone == tz {
			return nil
		}
		cj.Spec.TimeZone = &tz
		return k8sClient.Update(ctx, cj)
	})
}

// Apply reads TIMEZONE and NTP_SERVERS from the vjailbreak-settings ConfigMap
// and applies both to the host.
func Apply(ctx context.Context, k8sClient client.Client) (string, error) {
	settingsCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
		Name:      constants.VjailbreakSettingsConfigMapName,
		Namespace: constants.NamespaceMigrationSystem,
	}, settingsCM); err != nil {
		return "", fmt.Errorf("read vjailbreak-settings: %w", err)
	}

	rawTZ := strings.TrimSpace(settingsCM.Data["TIMEZONE"])
	rawNTP := strings.TrimSpace(settingsCM.Data["NTP_SERVERS"])
	ntpServers := FilterValidNTPServers(rawNTP)

	cleanTZ, tzErr := sanitizeTimezone(rawTZ)
	if tzErr != nil {
		logrus.Warnf("timesettings: %v, defaulting to UTC", tzErr)
		cleanTZ = ""
	}

	var (
		targetTZ    string
		syncEnabled bool
	)
	switch {
	case ntpServers != "":
		syncEnabled = true
		if cleanTZ != "" {
			targetTZ = cleanTZ
		} else {
			targetTZ = "UTC"
		}
	case cleanTZ != "":
		syncEnabled = true
		targetTZ = cleanTZ
	default:
		syncEnabled = false
		targetTZ = "UTC"
	}

	// === Sync layer ===
	if err := writeTimesyncdConf(ntpServers); err != nil {
		return "", fmt.Errorf("write timesyncd config: %w", err)
	}

	// === Best-effort follow-ups: aggregate, don't abort ===
	var errs []error
	if err := updatePf9EnvFile(targetTZ); err != nil {
		errs = append(errs, fmt.Errorf("update pf9 env file: %w", err))
	}

	if err := notifyTimedateViaDbus(targetTZ, syncEnabled); err != nil {
		errs = append(errs, fmt.Errorf("notify timedated: %w", err))
	}

	if syncEnabled {
		if err := restartTimesyncdViaDbus(); err != nil {
			errs = append(errs, fmt.Errorf("restart timesyncd: %w", err))
		}
	}
	if err := patchPf9EnvConfigMap(ctx, k8sClient, targetTZ); err != nil {
		errs = append(errs, fmt.Errorf("patch pf9-env configmap: %w", err))
	}
	if err := restartTZWorkloads(ctx, k8sClient); err != nil {
		errs = append(errs, fmt.Errorf("restart workloads: %w", err))
	}
	if err := patchVersionCheckerTZ(ctx, k8sClient, targetTZ); err != nil {
		errs = append(errs, fmt.Errorf("patch version-checker cronjob: %w", err))
	}

	logrus.Infof("timesettings: applied TIMEZONE=%q NTP_SERVERS=%q", targetTZ, ntpServers)
	if joined := errors.Join(errs...); joined != nil {
		logrus.Warnf("timesettings: non-fatal errors during apply (timezone=%s, ntp=%s): %v", targetTZ, ntpServers, joined)
	}
	return "Time settings applied successfully.", nil
}
