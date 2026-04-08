package cmd

import (
	"testing"
)

func TestTokensCommandRegistration(t *testing.T) {
	commands := rootCmd.Commands()

	var found bool
	for _, cmd := range commands {
		if cmd.Use == "tokens" {
			found = true
			break
		}
	}

	if !found {
		t.Error("tokens command not registered")
	}
}

func TestTokensSubcommands(t *testing.T) {
	commands := tokensCmd.Commands()

	commandNames := make(map[string]bool)
	for _, cmd := range commands {
		commandNames[cmd.Use] = true
	}

	requiredCommands := []string{"create", "list", "revoke"}
	for _, name := range requiredCommands {
		if !commandNames[name] {
			t.Errorf("tokens subcommand %q not registered", name)
		}
	}
}

func TestTokensCreateFlags(t *testing.T) {
	flag := tokensCreateCmd.Flags().Lookup("name")
	if flag == nil {
		t.Error("name flag not found on tokens create")
	}
}

func TestTokensRevokeFlags(t *testing.T) {
	flag := tokensRevokeCmd.Flags().Lookup("id")
	if flag == nil {
		t.Error("id flag not found on tokens revoke")
	}
}

func TestTokensCreateValidation_MissingName(t *testing.T) {
	// Reset required flags
	tokensCreateCmd.Flags().Set("name", "")

	err := tokensCreateCmd.RunE(tokensCreateCmd, []string{})

	if err == nil {
		t.Error("expected error for missing name")
	}

	expectedMsg := "--name is required"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want containing %q", err.Error(), expectedMsg)
	}
}

func TestTokensRevokeValidation_MissingID(t *testing.T) {
	// Reset required flags
	tokensRevokeCmd.Flags().Set("id", "")

	err := tokensRevokeCmd.RunE(tokensRevokeCmd, []string{})

	if err == nil {
		t.Error("expected error for missing id")
	}

	expectedMsg := "--id is required"
	if !contains(err.Error(), expectedMsg) {
		t.Errorf("error = %q, want containing %q", err.Error(), expectedMsg)
	}
}

func TestTokensCreateMissingClient(t *testing.T) {
	// Reset global vars
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	// Set valid name
	tokensCreateCmd.Flags().Set("name", "test-token")

	err := tokensCreateCmd.RunE(tokensCreateCmd, []string{})

	if err == nil {
		t.Error("expected error for missing client config")
	}
}

func TestTokensListMissingClient(t *testing.T) {
	// Reset global vars
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	err := tokensListCmd.RunE(tokensListCmd, []string{})

	if err == nil {
		t.Error("expected error for missing client config")
	}
}

func TestTokensRevokeMissingClient(t *testing.T) {
	// Reset global vars
	apiEndpoint = ""
	apiToken = ""
	orgID = ""
	env = ""

	// Set valid id
	tokensRevokeCmd.Flags().Set("id", "tok-123")

	err := tokensRevokeCmd.RunE(tokensRevokeCmd, []string{})

	if err == nil {
		t.Error("expected error for missing client config")
	}
}

func TestTokensCommandHelp(t *testing.T) {
	if tokensCmd.Short == "" {
		t.Error("tokens command missing short description")
	}
}

func TestTokensCreateCommandHelp(t *testing.T) {
	if tokensCreateCmd.Short == "" {
		t.Error("tokens create command missing short description")
	}
}

func TestTokensListCommandHelp(t *testing.T) {
	if tokensListCmd.Short == "" {
		t.Error("tokens list command missing short description")
	}
}

func TestTokensRevokeCommandHelp(t *testing.T) {
	if tokensRevokeCmd.Short == "" {
		t.Error("tokens revoke command missing short description")
	}
}
