package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bakhod1r/kyber/internal/chat/domain"
)

func TestConversationPref(t *testing.T) {
	p, err := domain.NewPref("ws", "u-1", "c-1", domain.LevelMuted, "  Projects ")
	if err != nil || p.Section != "Projects" || p.Level != domain.LevelMuted {
		t.Fatalf("pref = %+v %v", p, err)
	}
	if _, err := domain.NewPref("ws", "u-1", "c-1", "loud", ""); !errors.Is(err, domain.ErrInvalidPref) {
		t.Fatalf("bad level = %v", err)
	}
	if _, err := domain.NewPref("ws", "u-1", "c-1", domain.LevelAll, strings.Repeat("x", 41)); !errors.Is(err, domain.ErrInvalidPref) {
		t.Fatalf("long section = %v", err)
	}
	if def, _ := domain.NewPref("ws", "u-1", "c-1", "", ""); def.Level != domain.LevelMentions {
		t.Fatalf("default level = %q", def.Level)
	}
}

func TestSectionOrder(t *testing.T) {
	got, err := domain.SectionOrder([]string{" Work ", "Team", "Work"})
	if err != nil || len(got) != 2 || got[0] != "Work" || got[1] != "Team" {
		t.Fatalf("order = %v %v", got, err)
	}
	if _, err := domain.SectionOrder([]string{""}); !errors.Is(err, domain.ErrInvalidOrder) {
		t.Fatalf("empty name = %v", err)
	}
}
