package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"opencode-profiles/internal/paths"
)

// ErrGitNotFound 表示系统未安装 git。
var ErrGitNotFound = errors.New("git is not installed; please install git first")

// gitExec 缓存 exec.LookPath 结果，便于测试覆盖。
var gitExec = func() string {
	path, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	return path
}()

// Available 报告系统中是否安装了 git。
func Available() bool {
	return gitExec != ""
}

// run 在 profile 目录内执行 git 命令。返回 stdout、stderr。
func run(p *paths.Paths, name string, args ...string) (string, string, error) {
	if !Available() {
		return "", "", ErrGitNotFound
	}
	cmd := exec.Command(gitExec, args...)
	cmd.Dir = p.ProfileDir(name)
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err != nil {
		return out.String(), errOut.String(), err
	}
	return out.String(), errOut.String(), nil
}

// IsRepo 判断 profile 目录是否已初始化 git 仓库（存在 .git 目录）。
func IsRepo(p *paths.Paths, name string) bool {
	_, err := os.Stat(filepath.Join(p.ProfileDir(name), ".git"))
	return err == nil
}

// trackedFiles 返回 profile 中被 git 跟踪的三个文件，均相对 profile 目录。
var trackedFiles = []string{"opencode.json", "tui.json", "skills.yml"}

// Init 在 profile 目录初始化 git 仓库并做首次提交。
func Init(p *paths.Paths, name string) error {
	if !Available() {
		return ErrGitNotFound
	}
	if IsRepo(p, name) {
		return errors.New("profile '" + name + "' is already under version control")
	}
	gitignore := filepath.Join(p.ProfileDir(name), ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		if err := os.WriteFile(gitignore, []byte("skills/\n"), 0o644); err != nil {
			return err
		}
	}
	if _, _, err := run(p, name, "init", "-q"); err != nil {
		return err
	}
	// 用 os.Stat 过滤出实际存在的被跟踪文件再 add——git 的 --ignore-missing
	// 仅能与 --dry-run 搭配（git >= 2.43 强制），不能用于此处；tui.json/skills.yml
	// 可能不存在（如 -e 创建的 profile）。
	for _, f := range trackedFiles {
		if _, err := os.Stat(filepath.Join(p.ProfileDir(name), f)); err == nil {
			if _, _, err := run(p, name, "add", "--", f); err != nil {
				return err
			}
		}
	}
	if _, _, err := run(p, name, "add", "--", ".gitignore"); err != nil {
		return err
	}
	if _, _, err := run(p, name, "commit", "-m", "chore: initial commit for profile "+name); err != nil {
		return err
	}
	return nil
}

// errNotRepo 表示 profile 未启用版本管理。
func errNotRepo(name string) error {
	return errors.New("profile '" + name + "' is not under version control; run --git-init first")
}

// ensureRepo 返回 nil 或 errNotRepo。
func ensureRepo(p *paths.Paths, name string) error {
	if !Available() {
		return ErrGitNotFound
	}
	if !IsRepo(p, name) {
		return errNotRepo(name)
	}
	return nil
}

// Commit 提交 profile 的三个被跟踪文件，消息缺省时用自动消息。
func Commit(p *paths.Paths, name, message string) error {
	if err := ensureRepo(p, name); err != nil {
		return err
	}
	if message == "" {
		message = "chore: update profile " + name
	}
	// 用 os.Stat 过滤出实际存在的被跟踪文件再逐个 add——git 的 --ignore-missing
	// 仅能与 --dry-run 搭配（git >= 2.43 强制），不能用于此处；tui.json/skills.yml
	// 可能不存在（如 -e 创建的 profile）。
	for _, f := range trackedFiles {
		if _, err := os.Stat(filepath.Join(p.ProfileDir(name), f)); err == nil {
			if _, _, err := run(p, name, "add", "--", f); err != nil {
				return err
			}
		}
	}
	if _, _, err := run(p, name, "commit", "-m", message); err != nil {
		return err
	}
	return nil
}

// Log 返回 git log --oneline 输出（空仓库时返回 nil）。
func Log(p *paths.Paths, name string) (string, error) {
	if err := ensureRepo(p, name); err != nil {
		return "", err
	}
	out, _, err := run(p, name, "log", "--oneline")
	return out, err
}

// isClean 报告 profile 工作区是否有未提交改动。
func isClean(p *paths.Paths, name string) bool {
	out, _, err := run(p, name, "status", "--porcelain")
	return err == nil && strings.TrimSpace(out) == ""
}

// trackedAt 返回目标 commit 中实际存在的被跟踪配置文件。
// 避免对从未被跟踪的文件（如 -e 创建的 profile 缺 tui.json/skills.yml）
// 执行 checkout 时报 pathspec 错误。
func trackedAt(p *paths.Paths, name, commit string) ([]string, error) {
	out, _, err := run(p, name, "ls-tree", "-r", "--name-only", commit, "--", "opencode.json", "tui.json", "skills.yml")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(strings.TrimSpace(out), "\n") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

// Rollback 将工作区被跟踪文件恢复到指定 commit，保留提交历史。
// 工作区存在未提交改动时直接拒绝。
func Rollback(p *paths.Paths, name, commit string) error {
	if err := ensureRepo(p, name); err != nil {
		return err
	}
	if !isClean(p, name) {
		return errors.New("working tree has uncommitted changes; commit or stash first")
	}
	files, err := trackedAt(p, name, commit)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("commit '" + commit + "' tracks none of the profile config files")
	}
	args := append([]string{"checkout", commit, "--"}, files...)
	if _, _, err := run(p, name, args...); err != nil {
		return err
	}
	return nil
}
