package gateway

import (
	"net/http"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// A Claude refusal that names a model's own limit ("You've reached your
// Fable limit ... switch models with /model", YsiCing on X) rests that
// model on the account, whatever the usage reading holds or lacks: Opus
// and the rest were refused with it, though their quota was left. One that
// names no model ("You've hit your limit", "your usage limit") is still
// the account's.
func TestClaudeNamedModelLimitRestsThatModelOnly(t *testing.T) {
	fresh(t)
	const user = "named@example.com"
	acct := func(model string) candidate {
		return candidate{p: provider.Provider{ID: "claude", Account: &provider.Account{Agent: "claude", User: user}}, model: model, rest: "claude"}
	}
	s := &Server{}
	body := func(msg string) []byte {
		return []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"` + msg + `"}}`)
	}
	fable := acct("claude-fable-5-1")
	r := s.restAfter(fable, http.StatusTooManyRequests, http.Header{}, body("Claude Code: You've reached your Fable limit. Run /usage-credits to continue or switch models with /model."))
	if r.Why != failQuota || r.Model != "claude-fable-5-1" || r.Key != fable.restID() {
		t.Fatalf("rest: %+v", r)
	}
	for _, m := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-4-5"} {
		if _, ok := restingOf(acct(m)); ok {
			t.Fatalf("%s rests on an account whose Fable limit is reached", m)
		}
	}
	// a model other than the one named is refused with the account's words
	for _, msg := range []string{"You've hit your limit · resets 5:58am", "You've reached your usage limit."} {
		o := acct("claude-opus-5-5")
		if r := s.restAfter(o, http.StatusTooManyRequests, http.Header{}, body(msg)); r.Key != o.restKey() {
			t.Fatalf("%q rested %+v, want the account", msg, r)
		}
	}
	// and a limit named for another model's family is not this model's
	o := acct("claude-opus-5-5")
	if r := s.restAfter(o, http.StatusTooManyRequests, http.Header{}, body("You've reached your Sonnet limit.")); r.Key != o.restKey() {
		t.Fatalf("Sonnet's limit rested Opus as %+v, want the account", r)
	}
}

// Mythos draws on Anthropic's Fable limit pool: a 429 on claude-mythos-5-1
// naming the Fable limit rests that model alone (and writes a model-scoped
// limit notice), leaving Opus, Sonnet and Haiku un-rested on the account.
func TestClaudeMythosFableLimitRestsMythosOnly(t *testing.T) {
	fresh(t)
	const user = "mythos@example.com"
	acct := func(model string) candidate {
		return candidate{p: provider.Provider{ID: "claude", Account: &provider.Account{Agent: "claude", User: user}}, model: model, rest: "claude"}
	}
	s := &Server{}
	body := []byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Claude Code: You've hit your Fable limit · resets Nov 29, 4pm (UTC)"}}`)
	mythos := acct("claude-mythos-5-1")
	r := s.restAfter(mythos, http.StatusTooManyRequests, http.Header{}, body)
	if r.Why != failQuota || r.Model != "claude-mythos-5-1" || r.Key != mythos.restID() {
		t.Fatalf("rest: %+v, want model rest %q", r, mythos.restID())
	}
	if _, ok := restOf(mythos.restKey()); ok {
		t.Fatal("Fable limit on Mythos rested the whole account")
	}
	if n, ok := LimitNoticeOf(mythos.restKey()); ok {
		t.Fatalf("Fable limit on Mythos wrote account-level notice: %+v", n)
	}
	if n, ok := LimitNoticeOf(mythos.restID()); !ok || n.Model != "claude-mythos-5-1" {
		t.Fatalf("expected model-level limit notice for Mythos, got %+v ok=%v", n, ok)
	}
	for _, m := range []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-4-5"} {
		if got, ok := restingOf(acct(m)); ok {
			t.Fatalf("%s rests (%+v) on an account whose Fable limit was hit by Mythos", m, got)
		}
	}
}
