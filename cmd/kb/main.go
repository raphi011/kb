package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/raphi011/kb/internal/index"
	"github.com/raphi011/kb/internal/kb"
	"github.com/raphi011/kb/internal/server"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "kb",
		Short: "Git-backed markdown knowledge base",
	}

	root.AddCommand(indexCmd())
	root.AddCommand(searchCmd())
	root.AddCommand(listCmd())
	root.AddCommand(tagsCmd())
	root.AddCommand(linksCmd())
	root.AddCommand(backlinksCmd())
	root.AddCommand(catCmd())
	root.AddCommand(editCmd())
	root.AddCommand(serveCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func findRepoRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository (or any parent)")
		}
		dir = parent
	}
}

func openKB(repoPath string) (*kb.KB, error) {
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		repoPath, _ = findRepoRoot(cwd)
	}
	if repoPath == "" {
		repoPath = os.Getenv("KB_REPO")
	}
	if repoPath == "" {
		return nil, fmt.Errorf("not a git repository and KB_REPO not set")
	}
	dbPath := filepath.Join(repoPath, ".kb.db")
	return kb.Open(repoPath, dbPath)
}

func indexCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Index repository (full on first run, incremental after)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var repoPath string
			if len(args) > 0 {
				repoPath = args[0]
			}
			k, err := openKB(repoPath)
			if err != nil {
				return err
			}
			defer k.Close()
			return k.Index(full)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Force full reindex")
	return cmd
}

func searchCmd() *cobra.Command {
	var (
		tags        string
		limit       int
		interactive bool
	)
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search notes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			var tagFilter []string
			if tags != "" {
				tagFilter = strings.Split(tags, ",")
			}

			results, err := k.Search(strings.Join(args, " "), tagFilter)
			if err != nil {
				return err
			}

			if interactive {
				return fzfSelect(results)
			}

			for i, n := range results {
				if i >= limit {
					break
				}
				fmt.Printf("%s\t%s\t%s\n", n.Path, n.Title, strings.Join(n.Tags, ","))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&tags, "tags", "", "Filter by tags (comma-separated)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use fzf for selection")
	return cmd
}

func listCmd() *cobra.Command {
	var interactive bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			notes, err := k.AllNotes()
			if err != nil {
				return err
			}

			if interactive {
				return fzfSelect(notes)
			}

			for _, n := range notes {
				fmt.Printf("%s\t%s\n", n.Path, n.Title)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use fzf for selection")
	return cmd
}

func tagsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tags",
		Short: "List all tags with note counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			tags, err := k.AllTags()
			if err != nil {
				return err
			}
			for _, t := range tags {
				fmt.Printf("%s\t%d\n", t.Name, t.NoteCount)
			}
			return nil
		},
	}
}

func linksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "links <path>",
		Short: "Show outgoing links from a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			links, err := k.OutgoingLinks(args[0])
			if err != nil {
				return err
			}
			for _, l := range links {
				kind := "internal"
				if l.External {
					kind = "external"
				}
				fmt.Printf("[%s] %s → %s\n", kind, l.Title, l.TargetPath)
			}
			return nil
		},
	}
}

func backlinksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backlinks <path>",
		Short: "Show notes that link to this path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			links, err := k.Backlinks(args[0])
			if err != nil {
				return err
			}
			for _, l := range links {
				fmt.Printf("%s (%s)\n", l.SourcePath, l.SourceTitle)
			}
			return nil
		},
	}
}

func catCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cat <path>",
		Short: "Print raw markdown of a note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			content, err := k.ReadFile(args[0])
			if err != nil {
				return err
			}
			fmt.Print(string(content))
			return nil
		},
	}
}

func editCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Select a note with fzf and open in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			k, err := openKB("")
			if err != nil {
				return err
			}
			defer k.Close()

			notes, err := k.AllNotes()
			if err != nil {
				return err
			}

			var input strings.Builder
			for _, n := range notes {
				fmt.Fprintf(&input, "%s\t%s\n", n.Path, n.Title)
			}

			fzf := exec.Command("fzf", "--delimiter=\t", "--with-nth=2", "--preview="+fzfPreview())
			fzf.Stdin = strings.NewReader(input.String())
			fzf.Stderr = os.Stderr
			out, err := fzf.Output()
			if err != nil {
				var exitErr *exec.ExitError
				if errors.As(err, &exitErr) && exitErr.ExitCode() == 130 {
					return nil // user cancelled with Ctrl-C/Esc
				}
				return fmt.Errorf("fzf: %w", err)
			}

			path := strings.Split(strings.TrimSpace(string(out)), "\t")[0]
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			repoRoot, err := findRepoRoot(cwd)
			if err != nil {
				return fmt.Errorf("find repo root: %w", err)
			}
			editorCmd := exec.Command(editor, filepath.Join(repoRoot, path))
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			return editorCmd.Run()
		},
	}
}

func serveCmd() *cobra.Command {
	var (
		addr  string
		repo  string
		token string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start web server + git remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				return fmt.Errorf("--token is required")
			}

			repoPath := repo
			if repoPath == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				repoPath, _ = findRepoRoot(cwd)
			}
			if repoPath == "" {
				repoPath = os.Getenv("KB_REPO")
			}
			if repoPath == "" {
				return fmt.Errorf("not a git repository and KB_REPO not set")
			}

			k, err := openKB(repoPath)
			if err != nil {
				return err
			}
			defer k.Close()

			if err := k.Index(false); err != nil {
				return fmt.Errorf("index: %w", err)
			}

			srv, err := server.New(k, k, token, repoPath)
			if err != nil {
				return fmt.Errorf("create server: %w", err)
			}

			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			fmt.Printf("Listening on %s\n", addr)
			return srv.ListenAndServe(ctx, addr)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "Listen address")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository path (default: current dir)")
	cmd.Flags().StringVar(&token, "token", "", "Auth token (required)")
	return cmd
}

func fzfPreview() string {
	if _, err := exec.LookPath("bat"); err == nil {
		return "kb cat {1} | bat --style=plain --color=always --language=md"
	}
	return "kb cat {1}"
}

// fzfSelect pipes notes through fzf and prints the selected path to stdout.
func fzfSelect(notes []index.Note) error {
	if _, err := exec.LookPath("fzf"); err != nil {
		fmt.Fprintf(os.Stderr, "fzf not found, falling back to plain output\n")
		for _, n := range notes {
			fmt.Printf("%s\t%s\n", n.Path, n.Title)
		}
		return nil
	}

	var input strings.Builder
	for _, n := range notes {
		fmt.Fprintf(&input, "%s\t%s\t%s\n", n.Path, n.Title, strings.Join(n.Tags, ","))
	}

	fzf := exec.Command("fzf",
		"--delimiter=\t",
		"--with-nth=2..",
		"--preview="+fzfPreview(),
	)
	fzf.Stdin = strings.NewReader(input.String())
	fzf.Stderr = os.Stderr
	out, err := fzf.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 130 {
			return nil // user cancelled with Ctrl-C/Esc
		}
		return fmt.Errorf("fzf: %w", err)
	}

	path := strings.Split(strings.TrimSpace(string(out)), "\t")[0]
	fmt.Println(path)
	return nil
}
