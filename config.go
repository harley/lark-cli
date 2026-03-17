package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const defaultConfigPath = "~/.lark-cli/config.toml"

type cliConfig struct {
	Version  int                   `toml:"version"`
	Output   string                `toml:"output"`
	Lark     cliLarkConfig         `toml:"lark"`
	Profiles map[string]cliProfile `toml:"profiles"`
}

type cliProfile struct {
	Output string        `toml:"output"`
	Lark   cliLarkConfig `toml:"lark"`
}

type cliLarkConfig struct {
	AppIDEnv     string `toml:"app_id_env"`
	AppSecretEnv string `toml:"app_secret_env"`
	Domain       string `toml:"domain"`
	UserIDType   string `toml:"user_id_type"`
}

type activeCLIProfile struct {
	Name   string
	Output string
	Lark   cliLarkConfig
}

func resolveConfigPath(flagValue string) (string, bool) {
	trimmed := strings.TrimSpace(flagValue)
	if trimmed != "" {
		return trimmed, true
	}
	if envValue := strings.TrimSpace(os.Getenv("LARK_CLI_CONFIG")); envValue != "" {
		return envValue, true
	}
	return defaultConfigPath, false
}

func expandPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("path is empty")
	}
	if trimmed == "~" || strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		if trimmed == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(trimmed, "~/")), nil
	}
	return trimmed, nil
}

func loadCLIConfig(path string, required bool) (cliConfig, bool, error) {
	resolved, err := expandPath(path)
	if err != nil {
		return cliConfig{}, false, err
	}

	raw, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return cliConfig{}, false, nil
		}
		return cliConfig{}, false, fmt.Errorf("read config: %w", err)
	}

	var cfg cliConfig
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return cliConfig{}, true, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = map[string]cliProfile{}
	}

	return cfg, true, nil
}

func resolveActiveCLIProfile(cfg cliConfig, exists bool, requested string) (activeCLIProfile, error) {
	name := strings.TrimSpace(requested)
	if !exists {
		return defaultCLIProfile(name), nil
	}

	if name != "" {
		if name == "default" && hasLegacyCLIProfile(cfg) {
			return legacyCLIProfile(cfg), nil
		}
		if profile, ok := cfg.Profiles[name]; ok {
			return namedCLIProfile(cfg, name, profile), nil
		}
		return activeCLIProfile{}, fmt.Errorf("unknown profile %q (available: %s)", name, strings.Join(availableCLIProfiles(cfg), ", "))
	}

	if hasLegacyCLIProfile(cfg) {
		return legacyCLIProfile(cfg), nil
	}

	if len(cfg.Profiles) == 1 {
		names := availableNamedCLIProfiles(cfg)
		return namedCLIProfile(cfg, names[0], cfg.Profiles[names[0]]), nil
	}

	if profile, ok := cfg.Profiles["default"]; ok {
		return namedCLIProfile(cfg, "default", profile), nil
	}

	if len(cfg.Profiles) > 1 {
		return activeCLIProfile{}, fmt.Errorf("config defines multiple profiles; choose one with --profile (%s)", strings.Join(availableCLIProfiles(cfg), ", "))
	}

	return defaultCLIProfile(""), nil
}

func defaultCLIProfile(name string) activeCLIProfile {
	return activeCLIProfile{
		Name:   defaultCLIProfileName(name),
		Output: strings.TrimSpace(os.Getenv("LARK_OUTPUT")),
		Lark: cliLarkConfig{
			AppIDEnv:     defaultProfileAppIDEnv(name),
			AppSecretEnv: defaultProfileAppSecretEnv(name),
			Domain:       strings.TrimSpace(os.Getenv("LARK_DOMAIN")),
			UserIDType:   strings.TrimSpace(os.Getenv("LARK_USER_ID_TYPE")),
		},
	}
}

func legacyCLIProfile(cfg cliConfig) activeCLIProfile {
	active := defaultCLIProfile("")
	active.Name = "default"
	if trimmed := strings.TrimSpace(cfg.Output); trimmed != "" {
		active.Output = trimmed
	}
	active.Lark = mergeLegacyCLIConfig(active.Lark, cfg.Lark)
	return active
}

func namedCLIProfile(cfg cliConfig, name string, profile cliProfile) activeCLIProfile {
	active := defaultCLIProfile(name)
	if trimmed := strings.TrimSpace(cfg.Output); trimmed != "" {
		active.Output = trimmed
	}
	active.Lark.Domain = fallbackNonEmpty(strings.TrimSpace(cfg.Lark.Domain), active.Lark.Domain)
	active.Lark.UserIDType = fallbackNonEmpty(strings.TrimSpace(cfg.Lark.UserIDType), active.Lark.UserIDType)
	if trimmed := strings.TrimSpace(profile.Output); trimmed != "" {
		active.Output = trimmed
	}
	active.Lark = mergeNamedCLIConfig(active.Lark, profile.Lark)
	return active
}

func mergeLegacyCLIConfig(base cliLarkConfig, override cliLarkConfig) cliLarkConfig {
	base.AppIDEnv = fallbackNonEmpty(strings.TrimSpace(override.AppIDEnv), base.AppIDEnv)
	base.AppSecretEnv = fallbackNonEmpty(strings.TrimSpace(override.AppSecretEnv), base.AppSecretEnv)
	base.Domain = fallbackNonEmpty(strings.TrimSpace(override.Domain), base.Domain)
	base.UserIDType = fallbackNonEmpty(strings.TrimSpace(override.UserIDType), base.UserIDType)
	return base
}

func mergeNamedCLIConfig(base cliLarkConfig, override cliLarkConfig) cliLarkConfig {
	base.AppIDEnv = fallbackNonEmpty(strings.TrimSpace(override.AppIDEnv), base.AppIDEnv)
	base.AppSecretEnv = fallbackNonEmpty(strings.TrimSpace(override.AppSecretEnv), base.AppSecretEnv)
	base.Domain = fallbackNonEmpty(strings.TrimSpace(override.Domain), base.Domain)
	base.UserIDType = fallbackNonEmpty(strings.TrimSpace(override.UserIDType), base.UserIDType)
	return base
}

func hasLegacyCLIProfile(cfg cliConfig) bool {
	return strings.TrimSpace(cfg.Output) != "" ||
		strings.TrimSpace(cfg.Lark.AppIDEnv) != "" ||
		strings.TrimSpace(cfg.Lark.AppSecretEnv) != "" ||
		strings.TrimSpace(cfg.Lark.Domain) != "" ||
		strings.TrimSpace(cfg.Lark.UserIDType) != "" ||
		len(cfg.Profiles) == 0
}

func availableCLIProfiles(cfg cliConfig) []string {
	names := availableNamedCLIProfiles(cfg)
	if hasLegacyCLIProfile(cfg) && !slices.Contains(names, "default") {
		names = append(names, "default")
	}
	slices.Sort(names)
	return names
}

func availableNamedCLIProfiles(cfg cliConfig) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func defaultCLIProfileName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "default"
	}
	return trimmed
}

func defaultProfileAppIDEnv(name string) string {
	if prefix := profileEnvPrefix(name); prefix != "" {
		return prefix + "_LARK_APP_ID"
	}
	return "LARK_APP_ID"
}

func defaultProfileAppSecretEnv(name string) string {
	if prefix := profileEnvPrefix(name); prefix != "" {
		return prefix + "_LARK_APP_SECRET"
	}
	return "LARK_APP_SECRET"
}

func profileEnvPrefix(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || strings.EqualFold(trimmed, "default") {
		return ""
	}

	var b strings.Builder
	lastUnderscore := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - ('a' - 'A'))
			lastUnderscore = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}

	return strings.Trim(b.String(), "_")
}

func fallbackNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
