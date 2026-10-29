//go:build windows
// +build windows

package backend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRunPowerShellJSONBindsNamedArguments(t *testing.T) {
	// This checks argument binding, independently of host PowerShell startup
	// latency. Production callers retain their fixed eight-second deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := runPowerShellJSONContext(ctx, `param([string]$Value)
[pscustomobject]@{ value = $Value } | ConvertTo-Json -Compress`, "-Value", "bound value")
	if err != nil {
		t.Fatalf("runPowerShellJSON returned error: %v", err)
	}
	if value := strings.TrimSpace(string(output)); value != `{"value":"bound value"}` {
		t.Fatalf("PowerShell parameter binding output = %q, want %q", value, `{"value":"bound value"}`)
	}
}

func TestBrowserDirectoryObservationHandlesWholeArgumentQuotesAndFailsClosed(t *testing.T) {
	target := `C:\Users\Name With Space\AppData\ant-browser\data\profile-uuid`
	for _, command := range []string{
		`"C:\Program Files\Chrome\chrome.exe" "--user-data-dir=` + target + `"`,
		`"C:\Program Files\Chrome\chrome.exe" --user-data-dir="` + target + `"`,
		`"C:\Program Files\Chrome\chrome.exe" --user-data-dir "` + target + `"`,
	} {
		raw, _ := json.Marshal(map[string]any{"complete": true, "items": []browserUserDataProcess{{PID: 123, CommandLine: command}}})
		list, err := decodeBrowserDirectoryObservation(raw, target)
		if err != nil || len(list) != 1 || list[0].DebugPort != 0 {
			t.Fatalf("portless residual missed: %s %v", command, err)
		}
	}
	for _, raw := range []string{``, `null`, `{"complete":true}`, `{"complete":false,"items":[]}`, `{"complete":true,"items":[{"pid":12,"commandLine":"chrome.exe --user-data-dir"}]}`} {
		if _, err := decodeBrowserDirectoryObservation([]byte(raw), target); err == nil {
			t.Fatalf("partial proof accepted: %s", raw)
		}
	}
	if list, err := decodeBrowserDirectoryObservation([]byte(`{"complete":true,"items":[]}`), target); err != nil || len(list) != 0 {
		t.Fatal("explicit complete empty observation rejected")
	}
}

func TestRunPowerShellJSONHonorsExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	output, err := runPowerShellJSONContext(ctx, "throw 'expired query must not execute'")
	if err == nil || err.Error() != "powershell query timed out" || len(output) != 0 {
		t.Fatalf("expired query returned output=%q err=%v", output, err)
	}
}
