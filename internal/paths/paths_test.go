package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsDefaults(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := New("", "")
	want := filepath.Join(home, ".config", "opencode")
	if p.BaseDir() != want {
		t.Errorf("BaseDir = %q, want %q", p.BaseDir(), want)
	}
	if got := p.ConfigFile(); got != filepath.Join(want, "opencode.json") {
		t.Errorf("ConfigFile = %q", got)
	}
	if got := p.ProfilesDir(); got != filepath.Join(want, "profiles") {
		t.Errorf("ProfilesDir = %q", got)
	}
	if got := p.TUIConfigFile(); got != filepath.Join(want, "tui.json") {
		t.Errorf("TUIConfigFile = %q", got)
	}
	if got := p.ProfileDir("work"); got != filepath.Join(want, "profiles", "work") {
		t.Errorf("ProfileDir = %q", got)
	}
	if got := p.ProfileConfig("work"); got != filepath.Join(want, "profiles", "work", "opencode.json") {
		t.Errorf("ProfileConfig = %q", got)
	}
	if got := p.ProfileTUIConfig("work"); got != filepath.Join(want, "profiles", "work", "tui.json") {
		t.Errorf("ProfileTUIConfig = %q", got)
	}
	if got := p.ProfileSkills("work"); got != filepath.Join(want, "profiles", "work", "skills") {
		t.Errorf("ProfileSkills = %q", got)
	}
	if got := p.ProfileSkillsYML("work"); got != filepath.Join(want, "profiles", "work", "skills.yml") {
		t.Errorf("ProfileSkillsYML = %q", got)
	}
	if got := p.RelativeTarget("work"); got != filepath.Join("profiles", "work", "opencode.json") {
		t.Errorf("RelativeTarget = %q", got)
	}
	if got := p.RelativeTargetTUI("work"); got != filepath.Join("profiles", "work", "tui.json") {
		t.Errorf("RelativeTargetTUI = %q", got)
	}
}

func TestPathsSkillSources(t *testing.T) {
	home, _ := os.UserHomeDir()
	p := New("", "")
	if got := p.SkillSourcesDir(); got != filepath.Join(home, ".cc-switch", "skills") {
		t.Errorf("SkillSourcesDir = %q", got)
	}
	if got := p.SkillSource("rtk"); got != filepath.Join(home, ".cc-switch", "skills", "rtk") {
		t.Errorf("SkillSource = %q", got)
	}
}

func TestPathsCustom(t *testing.T) {
	base := filepath.Join(t.TempDir(), "opencode")
	src := filepath.Join(t.TempDir(), "my-skills")
	p := New(base, src)
	if p.BaseDir() != base {
		t.Errorf("BaseDir = %q, want %q", p.BaseDir(), base)
	}
	if p.SkillSourcesDir() != src {
		t.Errorf("SkillSourcesDir = %q, want %q", p.SkillSourcesDir(), src)
	}
}
