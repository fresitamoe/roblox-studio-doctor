package paths

import (
	"runtime"
	"strings"
)

func sep(goos string) string {
	if goos == "windows" {
		return `\`
	}
	return "/"
}

// filepath.Join builds paths for the OS this was compiled for, not the one
// mentioned throughout here. In that case, just do it by hand
func join(goos string, parts ...string) string {
	var cleaned []string
	for _, p := range parts {
		if t := strings.Trim(p, `/\`); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	prefix := ""
	if len(parts) > 0 && strings.HasPrefix(parts[0], "/") {
		prefix = "/"
	}
	return prefix + strings.Join(cleaned, sep(goos))
}

func base(p string) string {
	p = strings.TrimRight(p, `/\`)
	if i := strings.LastIndexAny(p, `/\`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// Candidates lists the log folders worth trying, most likely first
func Candidates(goos, home string, env func(string) string) []string {
	if env == nil {
		env = func(string) string { return "" }
	}
	var out []string
	add := func(p string) {
		if p != "" {
			out = append(out, p)
		}
	}

	switch goos {
	case "windows":
		if la := env("LOCALAPPDATA"); la != "" {
			add(join(goos, la, "Roblox", "logs"))
		}
		if home != "" {
			add(join(goos, home, "AppData", "Local", "Roblox", "logs"))
		}
	case "darwin":
		if home != "" {
			add(join(goos, home, "Library", "Logs", "Roblox"))
		}
	default:
		if home != "" {
			add(join(goos, home, ".var/app/org.vinegarhq.Vinegar/data/vinegar/appdata/Roblox/logs"))
			add(join(goos, home, ".var/app/org.vinegarhq.Sober/data/sober/appdata/Roblox/logs"))
			add(join(goos, home, ".local/share/vinegar/appdata/Roblox/logs"))
		}
		if wp := env("WINEPREFIX"); wp != "" {
			add(join(goos, wp, "drive_c", "users", base(home), "AppData", "Local", "Roblox", "logs"))
		}
	}
	return out
}

// Default is Candidates for whatever platform this is running on
func Default(home string, env func(string) string) []string {
	return Candidates(runtime.GOOS, home, env)
}
