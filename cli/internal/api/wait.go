package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Settled reports that nothing more happens without the caller acting:
// failed, held, a draft waiting for its portrait or voice, or ready / (v2
// manual) a candidate waiting to be picked, with no rebuild still capturing
// new candidates.
func Settled(a *Avatar) bool {
	switch a.Status {
	case "failed", "moderation_hold", "draft":
		return true
	}
	if a.Rebuilding != nil && *a.Rebuilding {
		return false
	}
	return a.Status == "ready" || (a.SelectionRequired != nil && *a.SelectionRequired)
}

// BuildFailedError is a build that ended failed or held for moderation.
type BuildFailedError struct{ Avatar *Avatar }

func (e *BuildFailedError) Error() string {
	a := e.Avatar
	if a.Status == "moderation_hold" {
		return fmt.Sprintf("avatar %s is held for moderation review", a.Id)
	}
	stage, feedback := "?", "no details"
	if a.LatestFailure != nil {
		stage = string(a.LatestFailure.Stage)
		if a.LatestFailure.UserFeedback != nil {
			feedback = *a.LatestFailure.UserFeedback
		}
	}
	return fmt.Sprintf("avatar %s failed at %s: %s", a.Id, stage, feedback)
}

// Wait polls until the avatar settles. onUpdate sees every changed state.
func (c *Client) Wait(ctx context.Context, avatarID string, timeout, interval time.Duration, onUpdate func(*Avatar)) (*Avatar, error) {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		a, err := c.Get(ctx, avatarID)
		if err != nil {
			return nil, err
		}
		if onUpdate != nil {
			// Compare values, not pointers (Rebuilding, stage feedback).
			s, _ := json.Marshal([]any{a.Status, a.Stages, a.Rebuilding})
			if string(s) != last {
				last = string(s)
				onUpdate(a)
			}
		}
		if Settled(a) {
			if a.Status == "failed" || a.Status == "moderation_hold" {
				return a, &BuildFailedError{a}
			}
			return a, nil
		}
		if time.Now().After(deadline) {
			return a, fmt.Errorf("avatar %s still %s after %s", avatarID, a.Status, timeout)
		}
		c.Sleep(interval)
	}
}

// Current is the candidates of the avatar's current version (its portrait,
// voice sample and preview message): the list can also show the serving take
// of an older version, and with versions=all every older version's takes.
// A server that does not send `current` is read by generation.
func Current(cl *CandidateList) []Candidate {
	var out []Candidate
	for _, x := range cl.Items {
		if x.Current != nil && *x.Current || x.Current == nil && x.Generation == cl.Generation {
			out = append(out, x)
		}
	}
	return out
}

// Previewable is the completed current-version candidates with a preview.
func Previewable(cl *CandidateList) []Candidate {
	var out []Candidate
	for _, x := range Current(cl) {
		if x.Status == "completed" && x.PreviewUrl != nil {
			out = append(out, x)
		}
	}
	return out
}

func pending(cl *CandidateList) int {
	n := 0
	for _, x := range Current(cl) {
		if x.Status == "pending" || x.Status == "processing" {
			n++
		}
	}
	return n
}

// WaitForCandidates polls until no current-version candidate is pending.
// With ids (the captures just requested), it waits until each of them is
// listed and settled, so a list read before they show up cannot end the wait.
func (c *Client) WaitForCandidates(ctx context.Context, avatarID string, timeout, interval time.Duration, ids ...string) (*CandidatesPage, error) {
	deadline := time.Now().Add(timeout)
	for {
		p, err := c.Candidates(ctx, avatarID)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 && settled(p.CandidateList, ids) || len(ids) == 0 && len(Current(p.CandidateList)) > 0 && pending(p.CandidateList) == 0 {
			return p, nil
		}
		if time.Now().After(deadline) {
			return p, fmt.Errorf("captures for %s still running after %s", avatarID, timeout)
		}
		c.Sleep(interval)
	}
}

// NewIDs are the captures a request just added: the current-version
// candidates in after (its response) that before (a list read just ahead of
// it) did not show. The response lists every current candidate, so older
// captures still queued are in it too; they do not count. When nothing is
// new, there are no ids, and a wait on them covers every current capture.
func NewIDs(before, after *CandidateList) []string {
	seen := map[string]bool{}
	for _, x := range before.Items {
		seen[x.CandidateId.String()] = true
	}
	cur := Current(after)
	var ids []string
	for _, x := range cur {
		if !seen[x.CandidateId.String()] {
			ids = append(ids, x.CandidateId.String())
		}
	}
	return ids
}

// settled: every id is listed and no longer pending or capturing.
func settled(cl *CandidateList, ids []string) bool {
	status := map[string]string{}
	for _, x := range cl.Items {
		status[x.CandidateId.String()] = string(x.Status)
	}
	for _, id := range ids {
		switch status[id] {
		case "", "pending", "processing":
			return false
		}
	}
	return true
}

// WaitServing polls until the avatar serves candidateID: the pick is recorded
// at once, and the avatar serves it once its config is built. It compares
// the ids rather than trusting a status flag, so an unrelated rebuild does
// not end it early, and a newer pick (by anyone) ends it with an error. Ids
// compare as UUIDs, so any spelling of candidateID matches.
func (c *Client) WaitServing(ctx context.Context, avatarID, candidateID string, timeout, interval time.Duration) (*Avatar, error) {
	want, err := ParseCandidateID(candidateID)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		a, err := c.Get(ctx, avatarID)
		if err != nil {
			return nil, err
		}
		switch {
		case a.Status == "failed" || a.Status == "moderation_hold":
			return a, &BuildFailedError{a}
		// Before the serving check: while a newer pick builds, the old one
		// still serves, and it is about to be replaced.
		case a.SelectedCandidateId != nil && *a.SelectedCandidateId != want:
			return a, fmt.Errorf("avatar %s: candidate %s was picked meanwhile", avatarID, a.SelectedCandidateId)
		case a.ServingCandidateId != nil && *a.ServingCandidateId == want:
			return a, nil
		}
		if time.Now().After(deadline) {
			return a, fmt.Errorf("avatar %s does not serve candidate %s after %s", avatarID, want, timeout)
		}
		c.Sleep(interval)
	}
}
