//go:build windows
// +build windows

package backend

import (
	"context"
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

func TestRunPowerShellJSONHonorsExpiredDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer cancel()
	output, err := runPowerShellJSONContext(ctx, "throw 'expired query must not execute'")
	if err == nil || err.Error() != "powershell query timed out" || len(output) != 0 {
		t.Fatalf("expired query returned output=%q err=%v", output, err)
	}
}
