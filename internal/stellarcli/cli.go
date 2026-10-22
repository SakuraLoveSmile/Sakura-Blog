// Package stellarcli 提供 Go 宿主的 Stellar 创作和诊断命令，不运行服务启动迁移。
package stellarcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"blog/internal/config"
	"blog/internal/stellarimages"
	"blog/internal/store"
)

type Environment struct {
	Adapter        string `json:"adapter"`
	Go             string `json:"go"`
	RequiredGo     string `json:"requiredGo"`
	Theme          string `json:"theme"`
	Commit         string `json:"commit"`
	ResourceSHA256 string `json:"resourceSHA256"`
	BaseDir        string `json:"baseDir"`
}

type Options struct {
	Root, ConfigPath string
	ResolvedApp      *config.App
	Environment      Environment
	Now              func() time.Time
	Images           func(context.Context, *config.App, *store.Store, stellarimages.PrepareOptions) (stellarimages.Report, error)
}

func ResolveApp(options Options) (*config.App, error) {
	if options.ResolvedApp != nil {
		snapshot := *options.ResolvedApp
		return &snapshot, nil
	}
	root := options.Root
	if root == "" {
		root = os.Getenv("BLOG_ROOT")
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	file := options.ConfigPath
	if file == "" {
		file = filepath.Join(root, "config", "app.yml")
	}
	app, err := config.LoadApp(file)
	if err != nil {
		return nil, err
	}
	if options.Root != "" {
		app.Root = options.Root
	} else if app.Root == "" {
		app.Root = root
	}
	return config.ResolveApp(app), nil
}

// Run 的退出码：0 成功，1 诊断或操作失败，2 无效命令；JSON stdout 不混入日志。
func Run(ctx context.Context, args []string, options Options, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintln(stderr, err); return 1 }
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: blog [-root <root>] stellar <doctor|new note|images>")
		return 2
	}
	app, err := ResolveApp(options)
	if err != nil {
		return fail(err)
	}
	flags := flag.NewFlagSet("stellar "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	var dryRun bool
	var format, notebook, title, tags, page, refresh string
	var author int64
	switch args[0] {
	case "doctor":
		flags.StringVar(&format, "format", "text", "text 或 json")
	case "new":
		if len(args) < 2 || args[1] != "note" {
			fmt.Fprintln(stderr, "Usage: stellar new note --notebook <id> --title <title> [--tags <tags>] [--author <id>] [--dry-run]")
			return 2
		}
		args = append([]string{args[0]}, args[2:]...)
		flags.StringVar(&notebook, "notebook", "", "Notebook id")
		flags.StringVar(&title, "title", "", "Note title")
		flags.StringVar(&tags, "tags", "", "Comma-separated tags")
		flags.Int64Var(&author, "author", 0, "Known author id (defaults to Notebook owner)")
		flags.BoolVar(&dryRun, "dry-run", false, "Read-only plan")
	case "images":
		flags.StringVar(&page, "page", "", "Generated page route")
		flags.StringVar(&refresh, "refresh", "", "Refresh image URL")
		flags.BoolVar(&dryRun, "dry-run", false, "No downloads or writes")
	default:
		fmt.Fprintln(stderr, "Unknown Stellar command:", args[0])
		return 2
	}
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Unexpected positional arguments")
		return 2
	}
	if args[0] == "doctor" && format != "text" && format != "json" {
		fmt.Fprintln(stderr, "--format 必须是 text 或 json")
		return 2
	}
	st, err := store.OpenExisting(app.DBPath, args[0] != "new" || dryRun)
	if args[0] == "doctor" {
		env := options.Environment
		env.BaseDir = app.Root
		result := Doctor(ctx, nil, env)
		if err != nil {
			// Doctor 的缺库诊断保持 stdout 可机器读取。
		} else {
			defer st.Close()
			result = Doctor(ctx, st, env)
		}
		if format == "json" {
			if err := writeJSON(stdout, result); err != nil {
				return fail(err)
			}
		} else {
			fmt.Fprintln(stdout, FormatDoctorText(result))
		}
		if !result.OK {
			return 1
		}
		return 0
	}
	if err != nil {
		return fail(err)
	}
	defer st.Close()
	if _, err := config.LoadStellarV2(st); err != nil {
		return fail(err)
	}
	if args[0] == "new" {
		now := time.Now()
		if options.Now != nil {
			now = options.Now()
		}
		plan, err := BuildNotePlan(st, NoteOptions{Notebook: notebook, Title: title, Tags: tags, Author: author, GeneratedAt: now, DBPath: app.DBPath, DryRun: dryRun})
		if err != nil {
			return fail(err)
		}
		if !dryRun {
			if err := WriteNotePlan(st, &plan); err != nil {
				return fail(err)
			}
		}
		if err := writeJSON(stdout, plan); err != nil {
			return fail(err)
		}
		return 0
	}
	if options.Images == nil {
		return fail(fmt.Errorf("stellar SSR image snapshot provider is unavailable"))
	}
	report, err := options.Images(ctx, app, st, stellarimages.PrepareOptions{Page: page, Refresh: refresh, DryRun: dryRun})
	if err != nil {
		return fail(err)
	}
	if err := writeJSON(stdout, report); err != nil {
		return fail(err)
	}
	return 0
}

func writeJSON(w io.Writer, value any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(value)
}
