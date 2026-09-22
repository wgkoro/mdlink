package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"mdlink/internal/diagnostic"
	"mdlink/internal/link"
	"mdlink/internal/markdown"
	"mdlink/internal/output"
	"mdlink/internal/query"
	"mdlink/internal/root"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func Run(args []string, stdout, stderr io.Writer) int { return run(args, os.Stdin, stdout, stderr) }

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return fail(stderr, 2, "command required")
	}
	if args[0] == "version" {
		if len(args) != 1 {
			return fail(stderr, 2, "version takes no arguments")
		}
		if _, err := fmt.Fprintf(stdout, "mdlink %s\ncommit: %s\nbuild date: %s\ngo: %s\n", Version, Commit, BuildDate, runtime.Version()); err != nil {
			fmt.Fprintln(stderr, "write version:", err)
			return 1
		}
		return 0
	}
	if args[0] != "outgoing" && args[0] != "backlinks" && args[0] != "unresolved" {
		return fail(stderr, 2, fmt.Sprintf("unknown command: %q", args[0]))
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	directory := flags.String("root", "", "Markdown root directory")
	counts := flags.Bool("counts", false, "include link counts")
	format := flags.String("format", "text", "output format: text or json")
	strict := flags.Bool("strict", false, "fail when diagnostics are present")
	checkFragments := false
	if command != "backlinks" {
		flags.BoolVar(&checkFragments, "check-fragments", false, "validate supported Wikilink headings and block IDs")
	}
	var sourceNames []string
	var sourceFile string
	explicitSources, explicitSourceFile := false, false
	if command == "unresolved" {
		flags.Func("source", "exact root-relative Markdown source (repeatable)", func(value string) error { explicitSources = true; sourceNames = append(sourceNames, value); return nil })
		flags.Func("sources0-from", "NUL-separated sources file, or - for stdin", func(value string) error {
			if explicitSourceFile || value == "" {
				return errors.New("sources0-from requires one nonempty file")
			}
			explicitSources = true
			explicitSourceFile = true
			sourceFile = value
			return nil
		})
	}
	var options root.Options
	flags.BoolVar(&options.NoGitignore, "no-gitignore", false, "disable root and nested .gitignore rules")
	explicitFollow := false
	flags.Func("follow-symlink", "logical symlink to follow (repeatable)", func(value string) error {
		explicitFollow = true
		options.FollowSymlinks = append(options.FollowSymlinks, value)
		return nil
	})
	flags.Func("exclude", "logical path to exclude (repeatable)", func(value string) error {
		options.Exclude = append(options.Exclude, value)
		return nil
	})
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	if err := flags.Parse(args[1:]); err == flag.ErrHelp {
		targetUsage := " TARGET"
		if command == "unresolved" {
			targetUsage = " [--source PATH] [--sources0-from FILE|-]"
		}
		if command != "backlinks" {
			targetUsage = " [--check-fragments]" + targetUsage
		}
		if _, err := fmt.Fprintf(stdout, "Usage: mdlink %s [--root DIR] [--follow-symlink PATH] [--exclude PATH] [--no-gitignore] [--counts] [--format text|json] [--strict]%s\n", command, targetUsage); err != nil {
			return fail(stderr, 1, "cannot write usage")
		}
		if _, err := fmt.Fprintln(stdout, ".gitignore rules apply by default within root (including nested files); --exclude adds literal paths. Git index/global rules are not read. Invalid patterns and POSIX named classes fail. See docs/limitations.md."); err != nil {
			return fail(stderr, 1, "cannot write usage")
		}
		if command != "backlinks" {
			if _, err := fmt.Fprintln(stdout, "--check-fragments validates a limited Wikilink heading/block-ID profile; Markdown anchors are unsupported. See docs/limitations.md."); err != nil {
				return fail(stderr, 1, "cannot write usage")
			}
		}
		return 0
	} else if err != nil {
		return fail(stderr, 2, fmt.Sprintf("invalid arguments: %q", err.Error()))
	}
	if command == "unresolved" && flags.NArg() != 0 {
		return fail(stderr, 2, "unresolved takes no target")
	}
	if command != "unresolved" && flags.NArg() != 1 {
		return fail(stderr, 2, command+" requires one target")
	}
	if *format != "text" && *format != "json" {
		return fail(stderr, 2, "format must be text or json")
	}
	if explicitSourceFile {
		input := stdin
		if sourceFile != "-" {
			f, e := os.Open(sourceFile)
			if e != nil {
				return fail(stderr, 1, "cannot read sources file")
			}
			defer f.Close()
			input = f
		}
		names, e := readSources0(input)
		if e != nil {
			code := 2
			if !errors.Is(e, errInvalidSources) {
				code = 1
			}
			return fail(stderr, code, "cannot read sources: "+e.Error())
		}
		sourceNames = append(sourceNames, names...)
	}
	for _, name := range sourceNames {
		if !validSourceName(name) {
			return fail(stderr, 2, fmt.Sprintf("invalid source: %q", name))
		}
	}
	explicitRoot := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "root" {
			explicitRoot = true
		}
	})
	physicalRoot, err := rootDirectory(*directory, explicitRoot)
	if err != nil {
		code := 1
		if errors.Is(err, errInvalidRoot) || errors.Is(err, os.ErrNotExist) {
			code = 2
		}
		return fail(stderr, code, "cannot select root")
	}
	if !explicitFollow {
		options.FollowSymlinks = filepath.SplitList(os.Getenv("MDLINK_FOLLOW_SYMLINKS"))
	}
	files, diagnostics, err := root.Walk(physicalRoot, options)
	if explicitSources || checkFragments {
		for i := range diagnostics {
			diagnostics[i].Phase = "catalog"
		}
	}
	if err != nil {
		if output.Diagnostics(stderr, diagnostics) != nil {
			return 1
		}
		if errors.Is(err, root.ErrInvalidOptions) {
			return fail(stderr, 2, "invalid root options")
		}
		return fail(stderr, 1, "cannot walk root")
	}
	catalog := root.NewCatalog(files)
	var selected []*root.File
	if explicitSources {

		var e error
		selected, sourceNames, e = selectSources(catalog, sourceNames)
		if e != nil {
			if output.Diagnostics(stderr, diagnostics) != nil {
				return 1
			}
			return fail(stderr, 2, e.Error())
		}
	}
	var results []query.Result
	var unresolved []query.UnresolvedResult
	var linkDiagnostics []diagnostic.Diagnostic
	var targetPath string
	if command == "unresolved" {
		unresolved, linkDiagnostics = query.Unresolved(catalog, selected, checkFragments)
	} else {
		target := link.Resolve(catalog, markdown.RawLink{RawTarget: flags.Arg(0), Kind: markdown.Wikilink})
		targetError := ""
		if target.Status != link.Resolved {
			targetError = "invalid target: " + string(target.Status)
		} else if command == "outgoing" && path.Ext(target.Target.LogicalPath) != ".md" {
			targetError = "outgoing target must be Markdown"
		}
		if targetError != "" {
			if output.Diagnostics(stderr, diagnostics) != nil {
				return 1
			}
			return fail(stderr, 2, targetError)
		}
		targetPath = target.Target.LogicalPath
		if command == "backlinks" {
			results, linkDiagnostics = query.Backlinks(catalog, *target.Target)
		} else {
			results, linkDiagnostics, err = query.Outgoing(catalog, *target.Target, checkFragments)
		}
	}
	if explicitSources || checkFragments {
		for i := range linkDiagnostics {
			if linkDiagnostics[i].Phase != "" {
				continue
			}
			linkDiagnostics[i].Phase = "source"
		}
	}
	diagnostics = append(diagnostics, linkDiagnostics...)
	if err != nil {
		if output.Diagnostics(stderr, diagnostics) != nil {
			return 1
		}
		return fail(stderr, 1, err.Error())
	}
	if *format == "json" {
		if command == "unresolved" {
			err = output.UnresolvedJSON(stdout, unresolved, diagnostics, sourceNames)
		} else {
			err = output.JSON(stdout, command, targetPath, results, diagnostics)
		}
		if err != nil {
			return fail(stderr, 1, "cannot write JSON results")
		}
	} else {
		if command == "unresolved" {
			for _, name := range sourceNames {
				if strings.ContainsAny(name, "\r\n\t") {
					return fail(stderr, 2, output.ErrTextPath.Error())
				}
			}
			err = output.UnresolvedText(stdout, unresolved, diagnostics, *counts)
		} else {
			err = output.Text(stdout, targetPath, results, diagnostics, *counts)
		}
		if err != nil {
			if errors.Is(err, output.ErrTextPath) {
				return fail(stderr, 2, err.Error())
			}
			return fail(stderr, 1, "cannot write results")
		}
		if output.Diagnostics(stderr, diagnostics) != nil {
			return 1
		}
	}
	if *strict && len(diagnostics) > 0 {
		return 3
	}
	return 0
}

func fail(stderr io.Writer, code int, message string) int {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return 1
	}
	return code
}
