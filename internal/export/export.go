// Package export 提供 profile 的导出与导入（zip 格式）。
package export

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
