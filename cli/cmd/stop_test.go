package cmd

import (
	"testing"
)

func TestRevokeCommandRegistration(t *testing.T) {
	var found bool
	for _, cmd := range sessionCmd.Commands() {
		if cmd.Use == "revoke" {
			found = true
			break
		}
	}

	if !found {
		t.Error("revoke command not registered under session")
	}
}

func TestRevokeCommandFlags(t *testing.T) {
	flag := sessionRevokeCmd.Flags().Lookup("session")
	if flag == nil {
		t.Error("session flag not found")
	}

	yesFlag := sessionRevokeCmd.Flags().Lookup("yes")
	if yesFlag == nil {
		t.Error("yes flag not found")
	}
}

func TestRevokeCommandHelp(t *testing.T) {
	if sessionRevokeCmd.Short == "" {
		t.Error("revoke command missing short description")
	}

	if sessionRevokeCmd.Long == "" {
		t.Error("revoke command missing long description")
	}
}

func TestRunRevokeValidation_MissingSessionID(t *testing.T) {
	// Reset flag
	revokeSessionID = ""
	revokeYes = true // Skip interactive prompt in tests

	err := runRevoke(sessionRevokeCmd, []string{})

	if err == nil {
		t.Error("expected error for missing session ID")
	}

	expectedMsg := "session ID required"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want containing %q", err.Error(), expectedMsg)
	}
}

func TestRevokeCommandUsage(t *testing.T) {
	expectedUse := "revoke"
	if sessionRevokeCmd.Use != expectedUse {
		t.Errorf("Use = %q, want %q", sessionRevokeCmd.Use, expectedUse)
	}
}

func TestRevokeCommandExample(t *testing.T) {
	// The long description contains the example
	if !contains(sessionRevokeCmd.Long, "trek session revoke") {
		t.Error("revoke command long description should contain usage example")
	}
}
