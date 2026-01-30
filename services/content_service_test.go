package services

import (
	"testing"

	"github.com/mcpx/boilerplate/stores"
)

func TestResolvePostGateRequiresLogin(t *testing.T) {
	post := &stores.PostModel{AccessLevel: stores.AccessLevelPublic}
	decision := resolvePostGate(post, nil, false)
	if !decision.IsLocked || decision.GateType != GateTypeLoginRequired {
		t.Fatalf("expected login gate, got %+v", decision)
	}
}

func TestResolvePostGateBlocksPaid(t *testing.T) {
	post := &stores.PostModel{AccessLevel: stores.AccessLevelPaid}
	viewer := &stores.UserModel{ID: 10}
	decision := resolvePostGate(post, viewer, false)
	if !decision.IsLocked || decision.GateType != GateTypePaywall {
		t.Fatalf("expected paywall gate, got %+v", decision)
	}
}

func TestResolvePostGateAllowsPaid(t *testing.T) {
	post := &stores.PostModel{AccessLevel: stores.AccessLevelPaid}
	viewer := &stores.UserModel{ID: 10}
	decision := resolvePostGate(post, viewer, true)
	if decision.IsLocked {
		t.Fatalf("expected unlocked post, got %+v", decision)
	}
}

func TestBuildTeaserMarkdown(t *testing.T) {
	markdown := "Para1\n\nPara2\n\nPara3"
	teaser := buildTeaserMarkdown(markdown)
	if teaser != "Para1\n\nPara2" {
		t.Fatalf("unexpected teaser: %q", teaser)
	}
}
