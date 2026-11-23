package presenter

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/72nd/sanity/store"
)

func loadedProject(t *testing.T) (*Project, *store.Fake) {
	t.Helper()
	st := store.NewFake()
	p := NewProject(st, stubT)
	if err := p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, st
}

func TestProjectLoadAndDirty(t *testing.T) {
	p, _ := loadedProject(t)
	if p.Name() != "Muster GmbH" || p.Inbox() != "office/inbox" || !strings.Contains(p.IgnoreText(), "DE123456789") {
		t.Fatalf("unexpected draft: %q %q %q", p.Name(), p.Inbox(), p.IgnoreText())
	}
	if p.Dirty() || p.StateText() != "project.state.saved" {
		t.Error("dirty after load")
	}
	p.SetName("Other")
	if !p.Dirty() || p.StateText() != "project.state.dirty" {
		t.Error("edit not dirty")
	}
	p.Revert()
	if p.Dirty() || p.Name() != "Muster GmbH" {
		t.Error("revert failed")
	}
	p.SetIgnoreText(p.IgnoreText() + "\n\n  ") // blank lines are not an edit
	if p.Dirty() {
		t.Error("blank lines count as edit")
	}
}

func TestProjectValidation(t *testing.T) {
	p, _ := loadedProject(t)
	p.SetName("  ")
	if p.Problems().Name == "" {
		t.Error("empty name accepted")
	}
	p.SetName("X")
	for _, in := range []string{"../other", "/abs/path", "a/../../b", "."} {
		p.SetInbox(in)
		if p.Problems().Inbox == "" {
			t.Errorf("inbox %q accepted", in)
		}
	}
	p.SetInbox("scans/in")
	p.SetIgnoreText("DE89 3704 0044 0532 0130 00\nDE123456789")
	if p.Problems().Any() {
		t.Errorf("valid IBAN rejected: %+v", p.Problems())
	}
	p.SetIgnoreText("DE123456789\nDE89 3704 0044 0532 0130 01")
	if pr := p.Problems(); pr.Ignore == "" {
		t.Error("IBAN with bad checksum accepted")
	}
	if _, err := p.Save(context.Background()); !errors.Is(err, ErrInvalidProject) {
		t.Errorf("Save with invalid IBAN: %v", err)
	}
}

func TestProjectSave(t *testing.T) {
	p, st := loadedProject(t)
	ctx := context.Background()
	p.SetName(" Neu AG ")
	p.SetInbox("scans/in")
	p.SetIgnoreText("A\n\n b \nA")
	changed, err := p.Save(ctx)
	if err != nil || !changed {
		t.Fatalf("Save = %v, %v", changed, err)
	}
	pr, _ := st.Project(ctx)
	if pr.Name != "Neu AG" || pr.Inbox != "scans/in" || !reflect.DeepEqual(pr.IgnoreTokens, []string{"A", "b"}) {
		t.Errorf("stored: %+v", pr)
	}
	if p.Dirty() {
		t.Error("dirty after save")
	}
	p.SetName("Neu 2")
	if changed, err := p.Save(ctx); err != nil || changed {
		t.Errorf("name-only save: changed=%v err=%v", changed, err)
	}
}

func TestProjectSetInboxFolder(t *testing.T) {
	p, _ := loadedProject(t)
	root := p.Root()
	rel, err := p.SetInboxFolder(filepath.Join(root, "a", "b"))
	if err != nil || rel != "a/b" || p.Inbox() != "a/b" {
		t.Errorf("inside root: %q %v", rel, err)
	}
	for _, bad := range []string{filepath.Dir(root), filepath.Join(filepath.Dir(root), "x"), root} {
		if _, err := p.SetInboxFolder(bad); !errors.Is(err, ErrOutsideRoot) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if p.Inbox() != "a/b" {
		t.Error("failed pick changed the inbox")
	}
}

func TestProjectStatementKeywords(t *testing.T) {
	p, st := loadedProject(t)
	ctx := context.Background()
	if !strings.Contains(p.StatementText(), "Kontoauszug") {
		t.Errorf("defaults missing: %q", p.StatementText())
	}
	p.SetStatementText("Kontoauszug, Extrait\n extrait \nRelevé")
	if !p.Dirty() {
		t.Error("not dirty")
	}
	if _, err := p.Save(ctx); err != nil {
		t.Fatal(err)
	}
	pr, _ := st.Project(ctx)
	if !reflect.DeepEqual(pr.StatementKeywords, []string{"Kontoauszug", "Extrait", "extrait", "Relevé"}) {
		t.Errorf("stored %v", pr.StatementKeywords)
	}
	if p.Dirty() {
		t.Error("dirty after save")
	}
}
