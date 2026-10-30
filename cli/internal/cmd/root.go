// Package cmd is the `atmee` command line.
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Atmanity/toolkit/cli/internal/api"
	"github.com/Atmanity/toolkit/cli/internal/creds"
)

// Version is set at build time (-ldflags "-X .../cmd.Version=..."); without
// that, it is the module version `go install ...@vX.Y.Z` records.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}

// usageError exits 2 (bad input), like a flag error.
type usageError struct{ error }

func usagef(format string, a ...any) error { return usageError{fmt.Errorf(format, a...)} }

// app is what every command shares; tests build one with their own streams.
type app struct {
	out, err io.Writer
	in       io.Reader
	store    creds.Store
	// mockScenario is the hidden --mock-scenario of the avatar commands.
	mockScenario string
}

func (a *app) baseURL() string {
	if u := strings.TrimSpace(os.Getenv("ATMEE_BASE_URL")); u != "" {
		return u
	}
	return api.DefaultBaseURL
}

func (a *app) api() (*api.Client, error) {
	cred, err := a.credential()
	if err != nil {
		return nil, err
	}
	c, err := api.New(a.baseURL(), cred, 60*time.Second)
	if err != nil {
		return nil, err
	}
	c.MockScenario = a.mockScenario
	c.JoinTimeout = joinTimeout
	c.WarmingTimeout = joinTimeout
	c.OnWarming = func(wait time.Duration) {
		a.note("The avatar is warming up on a rendering node; retrying in %ds\n", int(wait.Seconds()))
	}
	return c, nil
}

func (a *app) printJSON(v any) error {
	enc := json.NewEncoder(a.out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (a *app) printf(format string, args ...any) { fmt.Fprintf(a.out, format, args...) }

// note goes to stderr so --json output stays clean.
func (a *app) note(format string, args ...any) { fmt.Fprintf(a.err, format, args...) }

// prompt reads one line; hidden input when stdin is a terminal.
// confirm asks a yes/no question; anything but y or yes is "aborted".
func (a *app) confirm(question string) error {
	ans, err := a.prompt(question, false)
	if err != nil {
		return err
	}
	if !strings.EqualFold(ans, "y") && !strings.EqualFold(ans, "yes") {
		return usagef("aborted")
	}
	return nil
}

func (a *app) prompt(label string, hidden bool) (string, error) {
	fmt.Fprint(a.err, label+": ")
	if f, ok := a.in.(*os.File); ok && hidden && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(a.err)
		return strings.TrimSpace(string(b)), err
	}
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func ctx() context.Context { return context.Background() }

// pollInterval is how often waits poll; tests shorten it.
var pollInterval = 3 * time.Second

// joinTimeout bounds an avatar-only session start's join wait (per attempt)
// and, separately, its warm-up retries; tests shorten it.
var joinTimeout = api.JoinWaitTimeout

// NewRoot builds the command tree.
func NewRoot(out, errOut io.Writer, in io.Reader) *cobra.Command {
	a := &app{out: out, err: errOut, in: in, store: creds.Default()}
	root := &cobra.Command{
		Use:   "atmee",
		Short: "Atmee CLI — create and manage conversational avatars.",
		Long: "Atmee CLI — create conversational avatars and talk to them.\n\n" +
			"Every command calls the Atmee API at ATMEE_BASE_URL (default " + api.DefaultBaseURL + "; " +
			"a local mock is http://localhost:8091). Sign in first with " +
			"`atmee login` (browser), or use an API key (`atmee login --api-key` or ATMEE_API_KEY).",
		Example: `  atmee login
  atmee balance
  atmee avatars create --name Ada --image ./portrait.jpg --voice ./voice.m4a --wait
  atmee avatars session chat AVATAR_ID --open`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetIn(in)
	root.AddCommand(a.loginCmd(), a.whoamiCmd(), a.logoutCmd(), a.balanceCmd(), a.accountCmd(), a.redeemCmd(), a.keysCmd(), a.avatarsCmd(), a.speechCmd())
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute(args []string, out, errOut io.Writer, in io.Reader) int {
	root := NewRoot(out, errOut, in)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	fmt.Fprintf(errOut, "Error: %v\n", err)
	var u usageError
	if errors.As(err, &u) || strings.HasPrefix(err.Error(), "unknown ") || strings.Contains(err.Error(), "required flag") ||
		strings.Contains(err.Error(), "accepts ") || strings.Contains(err.Error(), "invalid argument") {
		return 2
	}
	return 1
}

// group is a command that only holds subcommands: an unknown one is a usage
// error (cobra would otherwise print help and exit 0).
func group(use, short string, subs ...*cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown command %q for %q", args[0], cmd.CommandPath())
			}
			return cmd.Help()
		},
	}
	c.AddCommand(subs...)
	return c
}
