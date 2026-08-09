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
