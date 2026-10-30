package cmd

import (
	"strings"
	"testing"
)

func TestAMissingArgumentIsNamedWithWhereToFindIt(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"avatars", "get"}, "missing AVATAR_ID (find it with `atmee avatars list`)"},
		{[]string{"avatars", "session", "chat"}, "missing AVATAR_ID"},
		{[]string{"avatars", "candidates", "select", "11111111-2222-4333-8444-555555555555"}, "missing CANDIDATE_ID (find it with `atmee avatars candidates list AVATAR_ID`)"},
		{[]string{"avatars", "session", "end"}, "missing SESSION_ID"},
		{[]string{"avatars", "replace-voice", "11111111-2222-4333-8444-555555555555"}, "missing PATH (a local file)"},
		{[]string{"avatars", "replace-knowledge"}, "missing AVATAR_ID"},
	}
	for _, tc := range cases {
		_, errOut, code := run(t, "", tc.args...)
		if code != 2 || !strings.Contains(errOut, tc.want) || !strings.Contains(errOut, "Usage: atmee ") {
			t.Errorf("%v: %d %q", tc.args, code, errOut)
		}
	}
	if _, errOut, code := run(t, "", "avatars", "get", "a", "b"); code != 2 || !strings.Contains(errOut, "too many arguments") {
		t.Errorf("extra argument: %d %q", code, errOut)
	}
}

func TestDeleteTakesYesAndStillTakesConfirm(t *testing.T) {
	isolate(t)
	for _, flag := range []string{"--yes", "--confirm"} {
		// No API is reachable: getting past the prompt to the request proves the flag skipped it.
		_, errOut, _ := run(t, "", "avatars", "delete", "11111111-2222-4333-8444-555555555555", flag)
		if strings.Contains(errOut, "aborted") || strings.Contains(errOut, "unknown flag") {
			t.Errorf("%s: %q", flag, errOut)
		}
	}
}
