package server

import (
	"context"
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
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timesyncdConfDir  = "/etc/systemd/timesyncd.conf.d"
	timesyncdConfFile = "/etc/systemd/timesyncd.conf.d/99-vjailbreak.conf"
	localtimePath     = "/etc/localtime"
	zoneinfoBase      = "/usr/share/zoneinfo"
	pf9EnvPath        = "/etc/pf9/env"
)

var deploymentsToRestart = []string{
	"migration-controller-manager",
	"migration-vpwned-sdk",
	"vjailbreak-ui",
}

func isValidNTPServer(s string) bool {
	if s == "" || strings.Contains(s, "://") || strings.Contains(s, "/") {
		return false
	}
	ipv4 := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if ipv4.MatchString(s) {
		for _, part := range strings.Split(s, ".") {
			v := 0
			fmt.Sscanf(part, "%d", &v)
			if v < 0 || v > 255 {
				return false
			}
		}
		return true
	}
	labelRE := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)
	if !regexp.MustCompile(`^[a-zA-Z0-9.-]+$`).MatchString(s) || strings.HasPrefix(s, ".") || strings.Contains(s, "..") {
		return false
	}
	for _, label := range strings.Split(s, ".") {
		if label == "" || len(label) > 63 || !labelRE.MatchString(label) {
			return false
		}
	}
	return true
}

func filterValidNTPServers(raw string) string {
	raw = strings.NewReplacer(",", " ", "\n", " ").Replace(raw)
	var valid, invalid []string
	for _, s := range strings.Fields(raw) {
		if isValidNTPServer(s) {
			valid = append(valid, s)
		} else {
			invalid = append(invalid, s)
		}
	}
	if len(invalid) > 0 {
		logrus.Warnf("applyTimeSettings: ignoring invalid NTP server entries: %v", invalid)
	}
	return strings.Join(valid, " ")
}

func writeTimesyncdConf(servers string) error {
	if err := os.MkdirAll(timesyncdConfDir, 0755); err != nil {
		return fmt.Errorf("create timesyncd conf dir: %w", err)
	}
	if servers == "" {
		if err := os.Remove(timesyncdConfFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove timesyncd conf: %w", err)
		}
		return nil
	}
	return os.WriteFile(timesyncdConfFile, []byte(fmt.Sprintf("[Time]\nNTP=%s\n", servers)), 0644)
}

func setLocaltimeSymlink(tz string) error {
	if tz == "" {
		tz = "UTC"
	}
	target := filepath.Join(zoneinfoBase, tz)
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("timezone %q not found in zoneinfo: %w", tz, err)
	}
	_ = os.Remove(localtimePath)
	return os.Symlink(target, localtimePath)
}

// notifyTimedateViaDbus uses the host's D-Bus to set the timezone and NTP state via systemd-timedated.
func notifyTimedateViaDbus(tz string, ntpEnabled bool) {
	conn, err := dbus.SystemBus()
	if err != nil {
		logrus.WithError(err).Warn("applyTimeSettings: cannot connect to system D-Bus (continuing)")
		return
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.timedate1", "/org/freedesktop/timedate1")
	if tz != "" {
		if call := obj.Call("org.freedesktop.timedate1.SetTimezone", 0, tz, false); call.Err != nil {
			logrus.WithError(call.Err).Warn("applyTimeSettings: D-Bus SetTimezone failed (file already updated)")
		}
	}
	if call := obj.Call("org.freedesktop.timedate1.SetNTP", 0, ntpEnabled, false); call.Err != nil {
		logrus.WithError(call.Err).Warn("applyTimeSettings: D-Bus SetNTP failed")
	}
}

// restartTimesyncdViaDbus restarts systemd-timesyncd so it picks up the new config.
func restartTimesyncdViaDbus() {
	conn, err := dbus.SystemBus()
	if err != nil {
		logrus.WithError(err).Warn("applyTimeSettings: cannot connect to D-Bus to restart timesyncd")
		return
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	var jobPath dbus.ObjectPath
	call := obj.Call("org.freedesktop.systemd1.Manager.RestartUnit", 0, "systemd-timesyncd.service", "replace")
	if call.Err != nil {
		logrus.WithError(call.Err).Warn("applyTimeSettings: D-Bus RestartUnit timesyncd failed")
		return
	}
	_ = call.Store(&jobPath)
}

func updatePf9EnvFile(tz string) {
	if tz == "" {
		tz = "UTC"
	}
	data, err := os.ReadFile(pf9EnvPath)
	if err != nil && !os.IsNotExist(err) {
		logrus.WithError(err).Warnf("applyTimeSettings: cannot read %s", pf9EnvPath)
		return
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
	if err := os.WriteFile(pf9EnvPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		logrus.WithError(err).Warnf("applyTimeSettings: cannot write %s", pf9EnvPath)
	}
}

func patchPf9EnvConfigMap(ctx context.Context, k8sClient client.Client, tz string) {
	if tz == "" {
		tz = "UTC"
	}
	cm := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
		Name:      "pf9-env",
		Namespace: constants.NamespaceMigrationSystem,
	}, cm); err != nil {
		logrus.WithError(err).Warn("applyTimeSettings: cannot get pf9-env configmap")
		return
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["TZ"] = tz
	if err := k8sClient.Update(ctx, cm); err != nil {
		logrus.WithError(err).Warn("applyTimeSettings: cannot update pf9-env configmap")
	}
}

func restartTZDeployments(ctx context.Context, k8sClient client.Client) {
	now := time.Now().Format(time.RFC3339)
	for _, name := range deploymentsToRestart {
		n := name
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			d := &appsv1.Deployment{}
			if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
				Name:      n,
				Namespace: constants.NamespaceMigrationSystem,
			}, d); err != nil {
				return client.IgnoreNotFound(err)
			}
			if d.Spec.Template.Annotations == nil {
				d.Spec.Template.Annotations = make(map[string]string)
			}
			d.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = now
			return k8sClient.Update(ctx, d)
		}); err != nil {
			logrus.WithError(err).Warnf("applyTimeSettings: cannot restart deployment %s", n)
		}
	}
}

func patchVersionCheckerTZ(ctx context.Context, k8sClient client.Client, tz string) {
	if tz == "" {
		tz = "UTC"
	}
	cj := &batchv1.CronJob{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
		Name:      "vjailbreak-version-checker",
		Namespace: constants.NamespaceMigrationSystem,
	}, cj); err != nil {
		return
	}
	cj.Spec.TimeZone = &tz
	if err := k8sClient.Update(ctx, cj); err != nil {
		logrus.WithError(err).Warn("applyTimeSettings: cannot patch version-checker cronjob timezone")
	}
}

func ApplyTimeSettingsOnHost(ctx context.Context, k8sClient client.Client) (string, error) {
	settingsCM := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, k8stypes.NamespacedName{
		Name:      constants.VjailbreakSettingsConfigMapName,
		Namespace: constants.NamespaceMigrationSystem,
	}, settingsCM); err != nil {
		return "", fmt.Errorf("read vjailbreak-settings: %w", err)
	}

	rawTZ := strings.TrimSpace(settingsCM.Data["TIMEZONE"])
	rawNTP := strings.TrimSpace(settingsCM.Data["NTP_SERVERS"])
	ntpServers := filterValidNTPServers(rawNTP)

	targetTZ := rawTZ
	if targetTZ != "" {
		if _, err := os.Stat(filepath.Join(zoneinfoBase, targetTZ)); err != nil {
			logrus.Warnf("applyTimeSettings: timezone %q not in zoneinfo, defaulting to UTC", targetTZ)
			targetTZ = "UTC"
		}
	}
	if ntpServers != "" && targetTZ == "" {
		targetTZ = "UTC"
	}

	if err := writeTimesyncdConf(ntpServers); err != nil {
		return "", fmt.Errorf("write timesyncd config: %w", err)
	}

	if targetTZ != "" {
		if err := setLocaltimeSymlink(targetTZ); err != nil {
			return "", fmt.Errorf("set /etc/localtime: %w", err)
		}
	}

	updatePf9EnvFile(targetTZ)

	ntpEnabled := targetTZ != "" || ntpServers != ""
	notifyTimedateViaDbus(targetTZ, ntpEnabled)
	if ntpEnabled {
		restartTimesyncdViaDbus()
	}

	patchPf9EnvConfigMap(ctx, k8sClient, targetTZ)
	restartTZDeployments(ctx, k8sClient)
	patchVersionCheckerTZ(ctx, k8sClient, targetTZ)

	logrus.Infof("applyTimeSettings: applied TIMEZONE=%q NTP_SERVERS=%q", targetTZ, ntpServers)
	return fmt.Sprintf("Time settings applied (timezone=%s, ntp=%s)", targetTZ, ntpServers), nil
}
