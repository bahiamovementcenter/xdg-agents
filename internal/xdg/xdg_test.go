package xdg

import "testing"

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaultsWhenUnset(t *testing.T) {
	d := Resolve(env(map[string]string{"HOME": "/home/u"}))
	want := map[Category]string{
		Config: "/home/u/.config",
		Data:   "/home/u/.local/share",
		State:  "/home/u/.local/state",
		Cache:  "/home/u/.cache",
	}
	for cat, path := range want {
		if d[cat].Path != path || d[cat].Source != "default" {
			t.Errorf("%s = %+v, want %s from default", cat, d[cat], path)
		}
	}
	// XDG_RUNTIME_DIR has no default
	if d[Runtime].Path != "" || d[Runtime].Source != "unset" {
		t.Errorf("runtime = %+v, want unset", d[Runtime])
	}
}

func TestEmptyIsUnset(t *testing.T) {
	d := Resolve(env(map[string]string{"HOME": "/home/u", "XDG_CONFIG_HOME": ""}))
	if d[Config].Path != "/home/u/.config" || d[Config].Source != "default" {
		t.Errorf("empty XDG_CONFIG_HOME: %+v", d[Config])
	}
}

func TestAbsoluteValueIsUsed(t *testing.T) {
	d := Resolve(env(map[string]string{
		"HOME": "/home/u", "XDG_STATE_HOME": "/var/state/u/", "XDG_RUNTIME_DIR": "/run/user/1000",
	}))
	if d[State].Path != "/var/state/u" || d[State].Source != "env" {
		t.Errorf("state = %+v", d[State])
	}
	if d[Runtime].Path != "/run/user/1000" || d[Runtime].Source != "env" {
		t.Errorf("runtime = %+v", d[Runtime])
	}
}

// A relative path is invalid and must be ignored: the default applies
func TestRelativeValueIsIgnored(t *testing.T) {
	d := Resolve(env(map[string]string{
		"HOME": "/home/u", "XDG_DATA_HOME": "my/data", "XDG_RUNTIME_DIR": "run",
	}))
	if d[Data].Path != "/home/u/.local/share" || d[Data].Source != "invalid" || d[Data].Ignored != "my/data" {
		t.Errorf("data = %+v", d[Data])
	}
	if d[Runtime].Path != "" || d[Runtime].Source != "invalid" {
		t.Errorf("runtime = %+v, want no path (no default) and the value ignored", d[Runtime])
	}
}
