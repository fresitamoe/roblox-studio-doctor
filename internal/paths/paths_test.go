package paths

import "testing"

func fakeEnv(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestCandidatesWindows(t *testing.T) {
	got := Candidates("windows", `C:\users\dev`, fakeEnv(map[string]string{
		"LOCALAPPDATA": `C:\users\dev\AppData\Local`,
	}))
	want := `C:\users\dev\AppData\Local\Roblox\logs`
	if len(got) == 0 || got[0] != want {
		t.Fatalf("got %v, want first element %q", got, want)
	}
}

func TestCandidatesWindowsNoEnv(t *testing.T) {
	got := Candidates("windows", `C:\users\dev`, fakeEnv(nil))
	for _, p := range got {
		if len(p) > 0 && (p[0] == '\\' || p[0] == '/') {
			t.Fatalf("bad path %q", p)
		}
	}
}

func TestCandidatesDarwin(t *testing.T) {
	got := Candidates("darwin", "/Users/dev", fakeEnv(nil))
	want := "/Users/dev/Library/Logs/Roblox"
	if len(got) == 0 || got[0] != want {
		t.Fatalf("got %v, want first element %q", got, want)
	}
}

func TestCandidatesVinegar(t *testing.T) {
	got := Candidates("linux", "/home/dev", fakeEnv(nil))
	want := "/home/dev/.var/app/org.vinegarhq.Vinegar/data/vinegar/appdata/Roblox/logs"
	found := false
	for _, p := range got {
		if p == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("vinegar path missing from %v", got)
	}
}

func TestCandidatesWinePrefix(t *testing.T) {
	got := Candidates("linux", "/home/dev", fakeEnv(map[string]string{
		"WINEPREFIX": "/home/dev/.wine",
	}))
	want := "/home/dev/.wine/drive_c/users/dev/AppData/Local/Roblox/logs"
	found := false
	for _, p := range got {
		if p == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("WINEPREFIX path missing from %v", got)
	}
}
