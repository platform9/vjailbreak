package targets

import (
	"errors"

	"github.com/platform9/vjailbreak/pkg/vpwned/openapiv3/proto/service/api"
)

var targets map[string]Targets = make(map[string]Targets)

type VMInfo struct {
	Name        string
	CPU         int64
	Memory      int64
	IPv4Addr    string
	IPv6Addr    string
	PowerStatus api.PowerStatus
	BootDevice  string
	GuestOS     string
}

type AccessInfo struct {
	Username     string
	Password     string
	HostnameOrIP string
	Port         string
}

type Targets interface {
	GetVM(a AccessInfo, name string) (VMInfo, error)
	ListVMs(a AccessInfo) ([]VMInfo, error)
	ReclaimVM(a AccessInfo, name string, args ...string) error
}

func RegisterTarget(name string, target Targets) {
	targets[name] = target
}

func DeleteTarget(name string) {
	delete(targets, name)
}

func GetTargets() []string {
	var names []string
	for name := range targets {
		names = append(names, name)
	}
	return names
}

func GetTarget(name string) (Targets, error) {
	target, ok := targets[name]
	if !ok {
		return nil, errors.New("target not found")
	}
	return target, nil
}
