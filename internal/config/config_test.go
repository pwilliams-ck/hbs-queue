package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	empty := func(string) string { return "" }
	cfg := Load(empty)

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Port", cfg.Port, "8080"},
		{"Env", cfg.Env, "localhost"},
		{"DebugPort", cfg.DebugPort, "6061"},
		{"SwaggerPort", cfg.SwaggerPort, "8081"},
		{"VCDVersion", cfg.VCDVersion, "38.0"},
		{"VCDOrg", cfg.VCDOrg, "System"},
		{"APIKey", cfg.APIKey, ""},
		{"DatabaseURL", cfg.DatabaseURL, ""},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"PORT":                         "9090",
		"ENV":                          "prod",
		"API_KEY":                      "secret",
		"DEBUG_PORT":                   "7071",
		"SWAGGER_PORT":                 "9091",
		"DATABASE_URL":                 "postgres://localhost/test",
		"VCD_URL":                      "https://vcd.example.com",
		"VCD_VERSION":                  "39.0",
		"VCD_USER":                     "admin",
		"VCD_PASSWORD":                 "pass",
		"VCD_ORG":                      "MyOrg",
		"HOOK_SECRET_DEBOARD_ORG":      "s1",
		"HOOK_SECRET_ONBOARD_CONTACT":  "s2",
		"HOOK_SECRET_DEBOARD_CONTACT":  "s3",
		"HOOK_SECRET_UPDATE_PW":        "s4",
		"HOOK_SECRET_UPDATE_BANDWIDTH": "s5",
	}
	getenv := func(key string) string { return env[key] }
	cfg := Load(getenv)

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Port", cfg.Port, "9090"},
		{"Env", cfg.Env, "prod"},
		{"APIKey", cfg.APIKey, "secret"},
		{"DebugPort", cfg.DebugPort, "7071"},
		{"SwaggerPort", cfg.SwaggerPort, "9091"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://localhost/test"},
		{"VCDBaseURL", cfg.VCDBaseURL, "https://vcd.example.com"},
		{"VCDVersion", cfg.VCDVersion, "39.0"},
		{"VCDUser", cfg.VCDUser, "admin"},
		{"VCDPassword", cfg.VCDPassword, "pass"},
		{"VCDOrg", cfg.VCDOrg, "MyOrg"},
		{"Hooks.DeboardOrg", cfg.Hooks.DeboardOrg, "s1"},
		{"Hooks.OnboardContact", cfg.Hooks.OnboardContact, "s2"},
		{"Hooks.DeboardContact", cfg.Hooks.DeboardContact, "s3"},
		{"Hooks.UpdatePW", cfg.Hooks.UpdatePW, "s4"},
		{"Hooks.UpdateBandwidth", cfg.Hooks.UpdateBandwidth, "s5"},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}
