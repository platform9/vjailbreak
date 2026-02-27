package utils

import (
	"testing"
)

func TestDetectScriptOSType(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		expected string
	}{
		{
			name: "Linux bash script with shebang",
			script: `#!/bin/bash
echo "Hello World"
apt-get update`,
			expected: ScriptOSTypeLinux,
		},
		{
			name: "Linux sh script with shebang",
			script: `#!/bin/sh
echo "Hello"`,
			expected: ScriptOSTypeLinux,
		},
		{
			name: "Windows batch script",
			script: `@echo off
REM This is a comment
echo Hello World
ipconfig`,
			expected: ScriptOSTypeWindows,
		},
		{
			name: "PowerShell script",
			script: `param($Name)
$env:PATH
Get-Service`,
			expected: ScriptOSTypeWindows,
		},
		{
			name: "Linux script with common commands",
			script: `echo "Starting"
systemctl start service
chmod 755 /tmp/file`,
			expected: ScriptOSTypeLinux,
		},
		{
			name: "Windows script with paths",
			script: `set PATH=C:\Windows\System32
echo %PROGRAMFILES%`,
			expected: ScriptOSTypeWindows,
		},
		{
			name: "Empty script",
			script: "",
			expected: ScriptOSTypeUnknown,
		},
		{
			name: "Ambiguous script",
			script: `echo "Hello"`,
			expected: ScriptOSTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectScriptOSType(tt.script)
			if result != tt.expected {
				t.Errorf("DetectScriptOSType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsScriptCompatibleWithOS(t *testing.T) {
	tests := []struct {
		name     string
		script   string
		osFamily string
		expected bool
	}{
		{
			name: "Linux script with Linux OS",
			script: `#!/bin/bash
apt-get update`,
			osFamily: "linuxGuest",
			expected: true,
		},
		{
			name: "Windows script with Windows OS",
			script: `@echo off
ipconfig`,
			osFamily: "windowsGuest",
			expected: true,
		},
		{
			name: "Linux script with Windows OS",
			script: `#!/bin/bash
apt-get update`,
			osFamily: "windowsGuest",
			expected: false,
		},
		{
			name: "Windows script with Linux OS",
			script: `@echo off
ipconfig`,
			osFamily: "linuxGuest",
			expected: false,
		},
		{
			name: "Unknown script type",
			script: `echo "Hello"`,
			osFamily: "linuxGuest",
			expected: true,
		},
		{
			name: "Empty script",
			script: "",
			osFamily: "linuxGuest",
			expected: true,
		},
		{
			name: "PowerShell with Windows",
			script: `$env:PATH`,
			osFamily: "windows2019Guest",
			expected: true,
		},
		{
			name: "Bash with CentOS",
			script: `#!/bin/bash
yum install`,
			osFamily: "centosGuest",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsScriptCompatibleWithOS(tt.script, tt.osFamily)
			if result != tt.expected {
				t.Errorf("IsScriptCompatibleWithOS() = %v, want %v (script type: %v)", 
					result, tt.expected, DetectScriptOSType(tt.script))
			}
		})
	}
}
