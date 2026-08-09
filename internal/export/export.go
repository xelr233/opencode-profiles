// Package export 提供 profile 的导出与导入（zip 格式）。
package export

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

// stripProviders 读取 opencode.json 内容并删除 provider 键，其余字段原样保留。
func stripProviders(raw []byte) ([]byte, error) {
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	delete(cfg, "provider")
	return json.MarshalIndent(cfg, "", "  ")
}

// Export 将 profile 导出为 zip。withSkills 时额外生成 <name>-skills.zip。
// warning 写入 warn。
func Export(p *paths.Paths, name, outDir string, withSkills bool, warn io.Writer) error {
	if err := ops.EnsureInitialized(p); err != nil {
		return err
	}
	configPath := p.ProfileConfig(name)
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("Profile '%s' not found", name)
	}
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	stripped, err := stripProviders(raw)
	if err != nil {
		return err
	}

	zipPath := filepath.Join(outDir, name+".zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)

	if err := writeZipBytes(zw, "opencode.json", stripped); err != nil {
		zw.Close()
		f.Close()
		return err
	}
	if _, err := os.Stat(p.ProfileSkillsYML(name)); err == nil {
		yml, err := os.ReadFile(p.ProfileSkillsYML(name))
		if err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if err := writeZipBytes(zw, "skills.yml", yml); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if _, err := os.Stat(p.ProfileTUIConfig(name)); err == nil {
		tui, err := os.ReadFile(p.ProfileTUIConfig(name))
		if err != nil {
			zw.Close()
			f.Close()
			return err
		}
		if err := writeZipBytes(zw, "tui.json", tui); err != nil {
			zw.Close()
			f.Close()
			return err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if withSkills {
		if err := exportSkillsZip(p, name, outDir, warn); err != nil {
			return err
		}
	}
	return nil
}

// writeZipBytes 向 zip writer 写入一个条目。
func writeZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// exportSkillsZip 导出 skills.yml 中引用的技能源为 <name>-skills.zip。缺失源跳过并 warning。
func exportSkillsZip(p *paths.Paths, name, outDir string, warn io.Writer) error {
	skillsList, err := skills.ReadSkillsYML(p, name)
	if err != nil {
		return err
	}
	zipPath := filepath.Join(outDir, name+"-skills.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	for _, skill := range skillsList {
		src := p.SkillSource(skill)
		if _, err := os.Stat(src); err != nil {
			fmt.Fprintf(warn, "Warning: skill source '%s' not found, skipped\n", skill)
			continue
		}
		err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			entry := skill + "/" + rel
			if d.IsDir() {
				_, err := zw.Create(entry + "/")
				return err
			}
			return addZipFile(zw, entry, path)
		})
		if err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}

// addZipFile 将文件内容写入 zip 的指定条目。
func addZipFile(zw *zip.Writer, entry, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	hdr.Name = entry
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	rc, err := os.Open(path)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(w, rc)
	return err
}

// Import 从 zip 还原 profile 到 profiles/<name>/。skillsZipPath 为空时尝试
// 同目录 <basename>-skills.zip 自动关联。warning 写入 warn。
func Import(p *paths.Paths, zipPath, name, skillsZipPath string, warn io.Writer) error {
	if err := ops.EnsureInitialized(p); err != nil {
		return err
	}
	profileDir := p.ProfileDir(name)
	if _, err := os.Stat(profileDir); err == nil {
		return fmt.Errorf("Profile '%s' already exists, use --name to import under a different name", name)
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	hasConfig := false
	for _, f := range zr.File {
		if f.Name == "opencode.json" {
			hasConfig = true
			break
		}
	}
	if !hasConfig {
		return fmt.Errorf("invalid profile zip '%s': missing opencode.json", zipPath)
	}

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		switch f.Name {
		case "opencode.json", "skills.yml", "tui.json":
			if err := extractFile(f, profileDir); err != nil {
				return err
			}
		}
	}

	// skills zip：显式指定或自动关联
	skillsPath := skillsZipPath
	if skillsPath == "" {
		base := strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
		candidate := filepath.Join(filepath.Dir(zipPath), base+"-skills.zip")
		if _, err := os.Stat(candidate); err == nil {
			skillsPath = candidate
		}
	}
	if skillsPath != "" {
		if err := importSkillsZip(p, skillsPath, warn); err != nil {
			return err
		}
	}

	// skills.yml 中引用的技能：本地缺失的 warning，存在的确保 symlink
	return linkExistingSkills(p, name, warn)
}

// extractFile 将 zip 条目解压到 dstDir，拒绝路径穿越。
func extractFile(f *zip.File, dstDir string) error {
	if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
		return fmt.Errorf("unsafe zip entry: %s", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dst := filepath.Join(dstDir, f.Name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, rc); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// importSkillsZip 解压 skills zip 到 skill_sources_dir。<skill> 源已存在时跳过该技能并 warning。
func importSkillsZip(p *paths.Paths, zipPath string, warn io.Writer) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		parts := strings.SplitN(f.Name, "/", 2)
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		skill := parts[0]
		src := p.SkillSource(skill)
		if _, err := os.Stat(src); err == nil {
			fmt.Fprintf(warn, "Warning: skill '%s' already exists locally, skipped\n", skill)
			continue
		}
		if strings.Contains(f.Name, "..") || filepath.IsAbs(f.Name) {
			return fmt.Errorf("unsafe zip entry: %s", f.Name)
		}
		rel := strings.TrimSuffix(parts[1], "/")
		if rel == "" {
			continue
		}
		dst := filepath.Join(src, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extractFile(f, src); err != nil {
			return err
		}
	}
	return nil
}

// linkExistingSkills 为 skills.yml 中本地存在的技能建立 base/skills/<skill> symlink。
// 本地缺失的技能打印 warning，不建 symlink；同名 symlink 已存在则跳过。
func linkExistingSkills(p *paths.Paths, name string, warn io.Writer) error {
	skillsList, err := skills.ReadSkillsYML(p, name)
	if err != nil {
		return err
	}
	skillsDir := filepath.Join(p.BaseDir(), "skills")
	for _, skill := range skillsList {
		if _, err := os.Stat(p.SkillSource(skill)); err != nil {
			fmt.Fprintf(warn, "Warning: skill '%s' not found locally, symlink not created\n", skill)
			continue
		}
		link := filepath.Join(skillsDir, skill)
		if _, err := os.Lstat(link); err == nil {
			continue
		}
		if err := os.MkdirAll(skillsDir, 0o755); err != nil {
			return err
		}
		if err := os.Symlink(p.SkillSource(skill), link); err != nil {
			return err
		}
	}
	return nil
}
