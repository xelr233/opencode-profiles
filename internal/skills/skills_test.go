package skills

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"opencode-profiles/internal/paths"
)

func newPaths(t *testing.T) (*paths.Paths, string) {
	t.Helper()
	src := filepath.Join(t.TempDir(), "skill-sources")
	for _, name := range []string{"brainstorming", "rtk", "mavenbuild"} {
		dir := filepath.Join(src, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := paths.New(filepath.Join(t.TempDir(), "opencode"), src)
	return p, filepath.Join(t.TempDir(), "nonexistent.db")
}

func TestReadWriteSkillsYml(t *testing.T) {
	p, _ := newPaths(t)
	got, err := ReadSkillsYML(p, "default")
	if err != nil || len(got) != 0 {
		t.Fatalf("ReadSkillsYML missing = %v, err = %v", got, err)
	}
	if err := WriteSkillsYML(p, "work", []string{"brainstorming", "rtk"}); err != nil {
		t.Fatal(err)
	}
	got, err = ReadSkillsYML(p, "work")
	if err != nil || len(got) != 2 || got[0] != "brainstorming" || got[1] != "rtk" {
		t.Fatalf("ReadSkillsYML = %v, err = %v", got, err)
	}
	if err := WriteSkillsYML(p, "test", []string{"mavenbuild"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.ProfileSkillsYML("test")); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkillsYML(p, "test", []string{"c"}); err != nil {
		t.Fatal(err)
	}
	got, _ = ReadSkillsYML(p, "test")
	if len(got) != 1 || got[0] != "c" {
		t.Fatalf("ReadSkillsYML overwrite = %v", got)
	}
}

func TestScanCurrentSkills(t *testing.T) {
	p, _ := newPaths(t)
	got, err := ScanCurrentSkills(p)
	if err != nil || len(got) != 0 {
		t.Fatalf("no dir = %v, err = %v", got, err)
	}
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rtk", "mavenbuild"} {
		if err := os.Symlink(p.SkillSource(name), filepath.Join(skillsDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	got, err = ScanCurrentSkills(p)
	if err != nil || len(got) != 2 || got[0] != "mavenbuild" || got[1] != "rtk" {
		t.Fatalf("symlinks = %v, err = %v", got, err)
	}
	if err := os.MkdirAll(filepath.Join(skillsDir, "real-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "file.txt"), []byte("not a skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ScanCurrentSkills(p)
	if err != nil || len(got) != 2 {
		t.Fatalf("ignores non-symlinks = %v, err = %v", got, err)
	}
}

func TestComputeDiff(t *testing.T) {
	cases := []struct {
		current, target, toAdd, toRemove []string
	}{
		{[]string{"a"}, []string{"a", "b", "c"}, []string{"b", "c"}, nil},
		{[]string{"a", "b", "c"}, []string{"a"}, nil, []string{"b", "c"}},
		{[]string{"a", "b"}, []string{"b", "c"}, []string{"c"}, []string{"a"}},
		{[]string{"a", "b"}, []string{"a", "b"}, nil, nil},
	}
	for _, c := range cases {
		toAdd, toRemove := ComputeDiff(c.current, c.target)
		if len(toAdd) != len(c.toAdd) || len(toRemove) != len(c.toRemove) {
			t.Fatalf("diff(%v,%v) = %v,%v want %v,%v", c.current, c.target, toAdd, toRemove, c.toAdd, c.toRemove)
		}
		for i := range toAdd {
			if toAdd[i] != c.toAdd[i] {
				t.Fatalf("toAdd = %v, want %v", toAdd, c.toAdd)
			}
		}
		for i := range toRemove {
			if toRemove[i] != c.toRemove[i] {
				t.Fatalf("toRemove = %v, want %v", toRemove, c.toRemove)
			}
		}
	}
}

func TestAddSkill(t *testing.T) {
	p, _ := newPaths(t)
	if err := AddSkill(p, "work", "rtk"); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadSkillsYML(p, "work")
	if len(got) != 1 || got[0] != "rtk" {
		t.Fatalf("add new = %v", got)
	}
	if err := AddSkill(p, "work", "rtk"); err != nil {
		t.Fatal(err)
	}
	got, _ = ReadSkillsYML(p, "work")
	if len(got) != 1 {
		t.Fatalf("no duplicate = %v", got)
	}
	if err := AddSkill(p, "work", "nonexistent"); err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestRemoveSkill(t *testing.T) {
	p, _ := newPaths(t)
	if err := WriteSkillsYML(p, "work", []string{"rtk", "mavenbuild"}); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSkill(p, "work", "rtk"); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadSkillsYML(p, "work")
	if len(got) != 1 || got[0] != "mavenbuild" {
		t.Fatalf("remove existing = %v", got)
	}
	if err := RemoveSkill(p, "work", "mavenbuild"); err != nil {
		t.Fatal(err)
	}
	got, _ = ReadSkillsYML(p, "work")
	if len(got) != 0 {
		t.Fatalf("remove not present = %v", got)
	}
}

func TestSyncSkills(t *testing.T) {
	p, db := newPaths(t)
	skillsDir := filepath.Join(p.BaseDir(), "skills")

	if err := WriteSkillsYML(p, "work", []string{"rtk", "mavenbuild"}); err != nil {
		t.Fatal(err)
	}
	if err := SyncSkills(p, "work", db); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rtk", "mavenbuild"} {
		fi, err := os.Lstat(filepath.Join(skillsDir, name))
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s not a symlink: %v", name, err)
		}
	}

	os.RemoveAll(skillsDir)
	os.MkdirAll(skillsDir, 0o755)
	for _, name := range []string{"rtk", "mavenbuild"} {
		if err := os.Symlink(p.SkillSource(name), filepath.Join(skillsDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteSkillsYML(p, "default", []string{"rtk", "mavenbuild"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkillsYML(p, "work", []string{"rtk"}); err != nil {
		t.Fatal(err)
	}
	if err := SyncSkills(p, "work", db); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "rtk")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "mavenbuild")); !os.IsNotExist(err) {
		t.Fatalf("mavenbuild should be removed, err = %v", err)
	}

	os.RemoveAll(skillsDir)
	if err := WriteSkillsYML(p, "work", []string{"rtk", "nonexistent"}); err != nil {
		t.Fatal(err)
	}
	if err := SyncSkills(p, "work", db); err == nil {
		t.Fatal("expected error for missing source")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "rtk")); !os.IsNotExist(err) {
		t.Fatalf("no partial sync: rtk should not exist, err = %v", err)
	}

	os.RemoveAll(skillsDir)
	os.MkdirAll(filepath.Join(skillsDir, "old-skill"), 0o755)
	if err := os.WriteFile(filepath.Join(skillsDir, "old-skill", "SKILL.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkillsYML(p, "default", nil); err != nil {
		t.Fatal(err)
	}
	if err := WriteSkillsYML(p, "work", []string{"rtk"}); err != nil {
		t.Fatal(err)
	}
	if err := SyncSkills(p, "work", db); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "old-skill")); !os.IsNotExist(err) {
		t.Fatalf("old-skill real dir should be removed, err = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(skillsDir, "rtk")); err != nil {
		t.Fatalf("rtk should be a symlink, err = %v", err)
	}
}

func TestUpdateDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE skills (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, directory TEXT NOT NULL,
		enabled_opencode BOOLEAN NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a", "b", "c"} {
		if _, err := db.Exec("INSERT INTO skills (id, name, directory) VALUES (?, ?, ?)",
			"local:"+name, name, name); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	if err := UpdateDB(dbPath, []string{"a", "c"}); err != nil {
		t.Fatal(err)
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name, enabled_opencode FROM skills ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []struct {
		name string
		flag int
	}
	for rows.Next() {
		var r struct {
			name string
			flag int
		}
		if err := rows.Scan(&r.name, &r.flag); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	want := []struct {
		name string
		flag int
	}{{"a", 1}, {"b", 0}, {"c", 1}}
	if len(got) != len(want) {
		t.Fatalf("rows = %+v", got)
	}
	for i := range want {
		if got[i].name != want[i].name || got[i].flag != want[i].flag {
			t.Fatalf("rows = %+v, want %+v", got, want)
		}
	}

	if err := UpdateDB(filepath.Join(t.TempDir(), "nonexistent.db"), []string{"a"}); err != nil {
		t.Fatalf("missing db should be skipped, err = %v", err)
	}
}
