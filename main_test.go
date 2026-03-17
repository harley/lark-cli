package main

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildUsersListPath(t *testing.T) {
	path := buildUsersListPath("open_department_id", "0", "open_id", "name,email", 50, "abc+/=")

	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse path: %v", err)
	}
	query := u.Query()

	if got := query.Get("department_id_type"); got != "open_department_id" {
		t.Fatalf("department_id_type = %q", got)
	}
	if got := query.Get("department_id"); got != "0" {
		t.Fatalf("department_id = %q", got)
	}
	if got := query.Get("user_id_type"); got != "open_id" {
		t.Fatalf("user_id_type = %q", got)
	}
	if got := query.Get("fields"); got != "name,email" {
		t.Fatalf("fields = %q", got)
	}
	if got := query.Get("page_size"); got != "50" {
		t.Fatalf("page_size = %q", got)
	}
	if got := query.Get("page_token"); got != "abc+/=" {
		t.Fatalf("page_token = %q", got)
	}
}

func TestUniqueUsers(t *testing.T) {
	input := []userProfile{
		{OpenID: "ou_1", Name: "Alpha"},
		{OpenID: "ou_1", Name: "Alpha Dup"},
		{UserID: "u_1", Name: "UserIDUser"},
		{UserID: "u_1", Name: "UserIDUser Dup"},
		{Email: "user@example.com", Name: "EmailUser"},
		{Email: "USER@example.com", Name: "EmailUser Dup"},
		{Name: "Fallback", Mobile: "123"},
		{Name: "Fallback", Mobile: "123"},
	}

	output := uniqueUsers(input)
	if len(output) != 5 {
		t.Fatalf("expected 5 unique users, got %d", len(output))
	}
}

func TestSortUsers(t *testing.T) {
	input := []userProfile{
		{Name: "zoe", Email: "z@example.com", OpenID: "ou_3"},
		{Name: "Anna", Email: "b@example.com", OpenID: "ou_2"},
		{Name: "anna", Email: "a@example.com", OpenID: "ou_1"},
	}

	sortUsers(input)

	if input[0].Email != "a@example.com" || input[1].Email != "b@example.com" || input[2].Email != "z@example.com" {
		t.Fatalf("unexpected sort order: %#v", input)
	}
}

func TestRedactToken(t *testing.T) {
	if got := redactToken("token123456789"); got != "token123...redacted" {
		t.Fatalf("unexpected redaction: %q", got)
	}
	if got := redactToken("abcd"); got != "***redacted" {
		t.Fatalf("unexpected short-token redaction: %q", got)
	}
}

func TestAppendQueryParams(t *testing.T) {
	path, err := appendQueryParams("/open-apis/im/v1/chats?existing=1", map[string]interface{}{
		"page_size": float64(5),
		"has_more":  true,
		"labels":    []interface{}{"a", "b"},
	})
	if err != nil {
		t.Fatalf("append query params: %v", err)
	}

	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse path: %v", err)
	}
	query := u.Query()

	if got := query.Get("existing"); got != "1" {
		t.Fatalf("existing = %q", got)
	}
	if got := query.Get("page_size"); got != "5" {
		t.Fatalf("page_size = %q", got)
	}
	if got := query.Get("has_more"); got != "true" {
		t.Fatalf("has_more = %q", got)
	}
	labels := query["labels"]
	if len(labels) != 2 || labels[0] != "a" || labels[1] != "b" {
		t.Fatalf("labels = %#v", labels)
	}
}

func TestAppendQueryParamsInvalidType(t *testing.T) {
	_, err := appendQueryParams("/open-apis/im/v1/chats", []interface{}{"bad"})
	if err == nil {
		t.Fatal("expected error for non-object params")
	}
}

func TestBuildUpdateMessageText(t *testing.T) {
	message, err := buildUpdateMessage("", "updated text", "", nil)
	if err != nil {
		t.Fatalf("build update message: %v", err)
	}
	if message.MsgType != "text" {
		t.Fatalf("msg type = %q", message.MsgType)
	}
	if message.Content.Text == nil || message.Content.Text.Text != "updated text" {
		t.Fatalf("unexpected text content: %#v", message.Content.Text)
	}
}

func TestBuildUpdateMessageJSON(t *testing.T) {
	message, err := buildUpdateMessage("", "", `{"msg_type":"interactive","content":{"card":{"config":{"update_multi":true}}}}`, nil)
	if err != nil {
		t.Fatalf("build update message from json: %v", err)
	}
	if message.MsgType != "interactive" {
		t.Fatalf("msg type = %q", message.MsgType)
	}
	if message.Content.Card == nil {
		t.Fatal("expected card content")
	}
}

func TestProfileEnvPrefix(t *testing.T) {
	if got := profileEnvPrefix("onboard-agent"); got != "ONBOARD_AGENT" {
		t.Fatalf("profileEnvPrefix = %q", got)
	}
	if got := profileEnvPrefix("default"); got != "" {
		t.Fatalf("default prefix = %q", got)
	}
}

func TestParseRootFlagsNamedProfileDefaults(t *testing.T) {
	t.Setenv("ONBOARD_LARK_APP_ID", "cli_onboard")
	t.Setenv("ONBOARD_LARK_APP_SECRET", "secret_onboard")

	cfg, remaining, err := parseRootFlags([]string{"--profile", "onboard", "version"})
	if err != nil {
		t.Fatalf("parseRootFlags: %v", err)
	}
	if len(remaining) != 1 || remaining[0] != "version" {
		t.Fatalf("remaining = %#v", remaining)
	}
	if cfg.Profile != "onboard" {
		t.Fatalf("profile = %q", cfg.Profile)
	}
	if cfg.AppID != "cli_onboard" || cfg.AppSecret != "secret_onboard" {
		t.Fatalf("unexpected creds = (%q, %q)", cfg.AppID, cfg.AppSecret)
	}
	if cfg.AppIDEnv != "ONBOARD_LARK_APP_ID" || cfg.AppSecretEnv != "ONBOARD_LARK_APP_SECRET" {
		t.Fatalf("unexpected env names = (%q, %q)", cfg.AppIDEnv, cfg.AppSecretEnv)
	}
}

func TestParseRootFlagsConfigProfile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	raw := `
version = 1
output = "text"

[profiles.forge]
output = "json"

[profiles.forge.lark]
app_id_env = "FORGE_LARK_APP_ID"
app_secret_env = "FORGE_LARK_APP_SECRET"
domain = "lark"
user_id_type = "open_id"
`
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("FORGE_LARK_APP_ID", "cli_forge")
	t.Setenv("FORGE_LARK_APP_SECRET", "secret_forge")

	cfg, _, err := parseRootFlags([]string{"--config", configPath, "--profile", "forge", "version"})
	if err != nil {
		t.Fatalf("parseRootFlags: %v", err)
	}
	if cfg.Profile != "forge" {
		t.Fatalf("profile = %q", cfg.Profile)
	}
	if cfg.OutputMode != "json" {
		t.Fatalf("output = %q", cfg.OutputMode)
	}
	if cfg.Domain != "lark" {
		t.Fatalf("domain = %q", cfg.Domain)
	}
	if cfg.UserIDType != "open_id" {
		t.Fatalf("user id type = %q", cfg.UserIDType)
	}
	if cfg.AppID != "cli_forge" || cfg.AppSecret != "secret_forge" {
		t.Fatalf("unexpected creds = (%q, %q)", cfg.AppID, cfg.AppSecret)
	}
}

func TestParseRootFlagsMultipleProfilesRequireSelection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	raw := `
version = 1

[profiles.alpha.lark]
app_id_env = "ALPHA_LARK_APP_ID"
app_secret_env = "ALPHA_LARK_APP_SECRET"

[profiles.beta.lark]
app_id_env = "BETA_LARK_APP_ID"
app_secret_env = "BETA_LARK_APP_SECRET"
`
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := parseRootFlags([]string{"--config", configPath, "version"})
	if err == nil {
		t.Fatal("expected profile selection error")
	}
}
