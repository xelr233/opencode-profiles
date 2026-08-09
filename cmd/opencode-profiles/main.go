package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"opencode-profiles/internal/diff"
	exportpkg "opencode-profiles/internal/export"
	gitpkg "opencode-profiles/internal/git"
	"opencode-profiles/internal/ops"
	"opencode-profiles/internal/paths"
	"opencode-profiles/internal/skills"
)

// version 通过 ldflags -X main.version=... 注入，缺省为 "dev"。
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, paths.New("", ""), ""))
}

// run 解析参数并执行命令。返回退出码；stdout/stderr 与 p 可注入供测试使用。
// dbPath 供测试注入（空字符串使用默认 ~/.cc-switch/cc-switch.db）。
func run(args []string, stdout, stderr io.Writer, p *paths.Paths, dbPath string) int {
	// 子命令：export / import（首个位置参数）
	if len(args) > 0 {
		switch args[0] {
		case "export":
			return runExport(args[1:], stdout, stderr, p)
		case "import":
			return runImport(args[1:], stdout, stderr, p)
		}
	}
	fs := flag.NewFlagSet("opencode-profiles", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		backupFlag      bool
		diffFlag        bool
		createName      string
		emptyName       string
		switchName      string
		listFlag        bool
		versionFlag     bool
		fromCurrent     bool
		fromProfile     string
		addSkillName    string
		removeSkillName string
		profileName     string
		gitInitName     string
		gitCommitName   string
		gitLogName      string
		gitRollbackName string
		commitMessage   string
	)
	fs.BoolVar(&backupFlag, "b", false, "备份当前配置")
	fs.BoolVar(&backupFlag, "backup", false, "备份当前配置")
	fs.BoolVar(&diffFlag, "d", false, "显示当前与目标 profile 的配置差异（可加 1-2 个 profile 名）")
	fs.BoolVar(&diffFlag, "diff", false, "显示配置差异")
	fs.StringVar(&createName, "c", "", "从当前配置创建新 profile")
	fs.StringVar(&createName, "create", "", "从当前配置创建新 profile")
	fs.StringVar(&emptyName, "e", "", "创建空 profile")
	fs.StringVar(&emptyName, "empty", "", "创建空 profile")
	fs.StringVar(&switchName, "s", "", "切换到指定 profile")
	fs.StringVar(&switchName, "switch", "", "切换到指定 profile")
	fs.BoolVar(&listFlag, "l", false, "列出所有 profile")
	fs.BoolVar(&listFlag, "list", false, "列出所有 profile")
	fs.BoolVar(&versionFlag, "version", false, "显示版本号")
	fs.BoolVar(&versionFlag, "v", false, "显示版本号")
	fs.BoolVar(&fromCurrent, "from-current", false, "从当前配置导入 provider（配合 -e 使用）")
	fs.StringVar(&fromProfile, "from-profile", "", "从指定 profile 导入 provider（配合 -e 使用）")
	fs.StringVar(&addSkillName, "add-skill", "", "Add a skill to a profile")
	fs.StringVar(&removeSkillName, "remove-skill", "", "Remove a skill from a profile")
	fs.StringVar(&profileName, "profile", "", "Target profile for --add-skill/--remove-skill")
	fs.StringVar(&gitInitName, "git-init", "", "Enable git version control for a profile")
	fs.StringVar(&gitCommitName, "git-commit", "", "Commit changes for a profile")
	fs.StringVar(&gitLogName, "git-log", "", "Show commit history for a profile")
	fs.StringVar(&gitRollbackName, "git-rollback", "", "Roll back a profile to a commit (soft)")
	fs.StringVar(&commitMessage, "m", "", "Commit message for --git-commit")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if versionFlag {
		fmt.Fprintf(stdout, "opencode-profiles %s\n", version)
		return 0
	}
	if !diffFlag && gitRollbackName == "" && fs.NArg() > 0 {
		fmt.Fprintln(stderr, "Unexpected argument: "+fs.Arg(0))
		return 2
	}
	if diffFlag {
		if backupFlag || createName != "" || emptyName != "" || switchName != "" || addSkillName != "" || removeSkillName != "" || listFlag || fromCurrent || fromProfile != "" {
			fmt.Fprintln(stderr, "Error: -d cannot be combined with other commands")
			return 1
		}
		for _, arg := range fs.Args() {
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintln(stderr, "Error: -d cannot be combined with other commands")
				return 1
			}
		}
		if fs.NArg() == 0 {
			fmt.Fprintln(stderr, "Error: -d requires at least one profile name")
			return 1
		}
		if fs.NArg() > 2 {
			fmt.Fprintln(stderr, "Error: -d accepts at most two profile names")
			return 1
		}
	}

	gitCmdCount := 0
	for _, s := range []string{gitInitName, gitCommitName, gitLogName, gitRollbackName} {
		if s != "" {
			gitCmdCount++
		}
	}
	if gitCmdCount > 1 {
		fmt.Fprintln(stderr, "Error: git commands are mutually exclusive")
		return 1
	}
	gitCmd := gitCmdCount > 0
	if gitCmd && (backupFlag || diffFlag || createName != "" || emptyName != "" || switchName != "" ||
		addSkillName != "" || removeSkillName != "" || listFlag || fromCurrent || fromProfile != "" || profileName != "") {
		fmt.Fprintln(stderr, "Error: git commands cannot be combined with other commands")
		return 1
	}
	if gitRollbackName != "" && fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: --git-rollback requires exactly one commit reference")
		return 1
	}
	if commitMessage != "" && gitCommitName == "" {
		fmt.Fprintln(stderr, "Error: -m can only be used with --git-commit")
		return 1
	}

	if fromCurrent && fromProfile != "" {
		fmt.Fprintln(stderr, "Error: --from-current and --from-profile are mutually exclusive")
		return 1
	}
	if (fromCurrent || fromProfile != "") && emptyName == "" {
		fmt.Fprintln(stderr, "Error: --from-current/--from-profile can only be used with -e")
		return 1
	}
	if fromProfile == "current" {
		fmt.Fprintln(stderr, "Error: 'current' is a reserved name and cannot be used as --from-profile value")
		return 1
	}

	if backupFlag {
		name, err := ops.Backup(p)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Backed up to '%s'\n", name)
	} else if createName != "" {
		if err := ops.CreateFromCurrent(p, createName); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Created profile '%s' from current config\n", createName)
	} else if emptyName != "" {
		var err error
		switch {
		case fromCurrent:
			err = ops.CreateEmpty(p, emptyName, "current")
		case fromProfile != "":
			err = ops.CreateEmpty(p, emptyName, fromProfile)
		default:
			err = ops.CreateEmpty(p, emptyName, "")
		}
		if err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		switch {
		case fromCurrent:
			fmt.Fprintf(stdout, "Created profile '%s' with providers from current config\n", emptyName)
		case fromProfile != "":
			fmt.Fprintf(stdout, "Created profile '%s' with providers from '%s'\n", emptyName, fromProfile)
		default:
			fmt.Fprintf(stdout, "Created empty profile '%s'\n", emptyName)
		}
	} else if diffFlag {
		var a, b string
		if fs.NArg() == 1 {
			a = ops.GetActive(p)
			if a == "" {
				fmt.Fprintln(stderr, "Error: no active profile to diff against")
				return 1
			}
			b = fs.Arg(0)
		} else {
			a, b = fs.Arg(0), fs.Arg(1)
		}
		result, err := diff.Diff(p, a, b)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		diff.Render(stdout, result)
	} else if gitInitName != "" {
		if err := gitpkg.Init(p, gitInitName); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Version control enabled for '%s'\n", gitInitName)
	} else if gitCommitName != "" {
		if err := gitpkg.Commit(p, gitCommitName, commitMessage); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Committed changes for '%s'\n", gitCommitName)
	} else if gitLogName != "" {
		log, err := gitpkg.Log(p, gitLogName)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintln(stdout, log)
	} else if gitRollbackName != "" {
		if err := gitpkg.Rollback(p, gitRollbackName, fs.Arg(0)); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Rolled back '%s' to %s; changes are staged, run --git-commit to record\n", gitRollbackName, fs.Arg(0))
	} else if switchName != "" {
		if err := ops.SwitchDB(p, switchName, dbPath, stdout); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Switched to '%s'\n", switchName)
	} else if addSkillName != "" {
		if profileName == "" {
			fmt.Fprintln(stderr, "Error: --add-skill requires --profile")
			return 1
		}
		if err := skills.AddSkill(p, profileName, addSkillName); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Added skill '%s' to profile '%s'\n", addSkillName, profileName)
	} else if removeSkillName != "" {
		if profileName == "" {
			fmt.Fprintln(stderr, "Error: --remove-skill requires --profile")
			return 1
		}
		if err := skills.RemoveSkill(p, profileName, removeSkillName); err != nil {
			fmt.Fprintf(stderr, "Error: %s\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Removed skill '%s' from profile '%s'\n", removeSkillName, profileName)
	} else if listFlag {
		profiles := ops.ListProfiles(p)
		active := ops.GetActive(p)
		if len(profiles) == 0 {
			fmt.Fprintln(stdout, "No profiles found.")
			return 0
		}
		for _, name := range profiles {
			if name == active {
				fmt.Fprintf(stdout, "  %s *\n", name)
			} else {
				fmt.Fprintf(stdout, "  %s\n", name)
			}
		}
		if active != "" {
			fmt.Fprintf(stdout, "\nActive: %s\n", active)
		}
	} else {
		fmt.Fprintln(stdout, "Use --help for available commands.")
	}
	return 0
}

// runExport 处理 export 子命令：export <name> [--with-skills] [--out <dir>]。
func runExport(args []string, stdout, stderr io.Writer, p *paths.Paths) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	withSkills := fs.Bool("with-skills", false, "也导出源 skills 压缩包")
	outDir := fs.String("out", ".", "输出目录（默认当前目录）")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: export requires exactly one profile name")
		return 1
	}
	name := fs.Arg(0)
	if err := exportpkg.Export(p, name, *outDir, *withSkills, stderr); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Exported profile '%s' to %s\n", name, filepath.Join(*outDir, name+".zip"))
	return 0
}

// runImport 处理 import 子命令：import <file.zip> [--name <new>] [--skills <file.zip>]。
func runImport(args []string, stdout, stderr io.Writer, p *paths.Paths) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "导入后的 profile 名（默认取 zip 文件名）")
	skillsZip := fs.String("skills", "", "显式指定 skills zip")
	if err := fs.Parse(reorderFlags(args, fs)); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: import requires exactly one zip file")
		return 1
	}
	zipPath := fs.Arg(0)
	importName := *name
	if importName == "" {
		importName = strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath))
	}
	if err := exportpkg.Import(p, zipPath, importName, *skillsZip, stderr); err != nil {
		fmt.Fprintf(stderr, "Error: %s\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Imported profile '%s' from %s\n", importName, zipPath)
	return 0
}

// reorderFlags 将位置参数移到末尾，使 stdlib flag 包能解析与位置参数混排的选项
// （flag 包在第一个非 flag 参数处停止解析）。带取值（非 bool）的 flag 连取其下一参数。
func reorderFlags(args []string, fs *flag.FlagSet) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.Index(name, "="); eq != -1 {
				name = name[:eq]
			}
			if f := fs.Lookup(name); f != nil && !isBoolFlag(f) {
				if i+1 < len(args) {
					flags = append(flags, args[i+1])
					i++
				}
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

// isBoolFlag 判断 flag 是否为 bool 类型（bool flag 不带取值）。
func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}
