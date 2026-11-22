package presenter

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/72nd/sanity/store"
)

func newTypes(t *testing.T) (*Types, *store.Fake) {
	t.Helper()
	st := store.NewFake()
	p := NewTypes(st, stubT)
	if err := p.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p, st
}

func TestTypesLoadSelectsFirst(t *testing.T) {
	p, _ := newTypes(t)
	if len(p.List()) != 5 {
		t.Fatalf("types = %d", len(p.List()))
	}
	cur, ok := p.Current()
	if !ok || cur.ID != p.List()[0].ID {
		t.Fatalf("current = %+v %v", cur, ok)
	}
	if p.Root() == "" {
		t.Error("root missing")
	}
	if len(p.Rules()) == 0 || p.Dirty() {
		t.Error("rules empty or dirty after load")
	}
}

func TestTypesEditSaveRevert(t *testing.T) {
	ctx := context.Background()
	p, st := newTypes(t)
	p.SetName("Renamed")
	if !p.Dirty() {
		t.Fatal("not dirty")
	}
	p.Revert()
	if p.Dirty() {
		t.Fatal("dirty after revert")
	}
	p.SetName("Renamed")
	p.SetThreshold(7)
	p.SetPreferredLabels(" a , ,b ")
	if err := p.SetDateSource("filename"); err != nil {
		t.Fatal(err)
	}
	if err := p.Save(ctx); err != nil {
		t.Fatal(err)
	}
	if p.Dirty() {
		t.Error("dirty after save")
	}
	got, _ := st.Types(ctx)
	if got[0].Name != "Renamed" || got[0].Threshold != 1 || got[0].Date.Source != "filename" ||
		strings.Join(got[0].Date.PreferredLabels, "|") != "a|b" {
		t.Errorf("stored = %+v", got[0])
	}
	if err := p.SetDateSource("nonsense"); err == nil {
		t.Error("bad source accepted")
	}
}

func TestTypesAddDuplicateDelete(t *testing.T) {
	ctx := context.Background()
	p, st := newTypes(t)

	p.Add()
	if !p.Dirty() || len(p.List()) != 6 {
		t.Fatalf("add: dirty=%v rows=%d", p.Dirty(), len(p.List()))
	}
	if err := p.Save(ctx); err == nil {
		t.Fatal("saved a type without name")
	}
	p.SetName("Strom")
	p.SetPathTemplate("receipts/{date:2006}/{date:2006-01-02}_strom.pdf")
	if err := p.Save(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Types(ctx); len(got) != 6 || got[5].ID != "strom" {
		t.Fatalf("stored = %+v", got)
	}

	if err := p.Duplicate(); err != nil {
		t.Fatal(err)
	}
	cur, _ := p.Current()
	if cur.ID == "strom" || !strings.HasPrefix(cur.Name, "types.copy.name") {
		t.Errorf("duplicate = %+v", cur)
	}
	if err := p.Save(ctx); err != nil {
		t.Fatal(err)
	}

	if err := p.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Types(ctx); len(got) != 6 {
		t.Errorf("after delete: %d types", len(got))
	}
	// A discarded new draft is not deleted from the store.
	p.Add()
	if err := p.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Types(ctx); len(got) != 6 || len(p.List()) != 6 {
		t.Errorf("draft delete touched the store: %d/%d", len(got), len(p.List()))
	}
}

func TestTypesRules(t *testing.T) {
	p, _ := newTypes(t)
	n := len(p.Rules())
	i := p.AddRule()
	if len(p.Rules()) != n+1 || i != n {
		t.Fatal("add rule")
	}
	if err := p.SetRuleValue(i, "  "); err == nil {
		t.Error("empty value accepted")
	}
	if err := p.SetRuleKind(i, "regex"); err != nil {
		t.Fatal(err)
	}
	if err := p.SetRuleValue(i, "(unclosed"); err == nil {
		t.Error("bad regex accepted")
	}
	if err := p.SetRuleValue(i, `W\d{9}`); err != nil {
		t.Error(err)
	}
	if err := p.SetRuleStrength(i, "strong"); err != nil || p.Rules()[i].Rule.Weight != 0.9 {
		t.Errorf("strength: %v %v", err, p.Rules()[i].Rule.Weight)
	}
	_ = p.SetRuleMode(i, "not")
	_ = p.SetRuleWhere(i, "footer")
	p.SetRuleEnabled(i, false)
	r := p.Rules()[i].Rule
	if r.Mode != "not" || r.Where != "footer" || r.Enabled {
		t.Errorf("rule = %+v", r)
	}
	if err := p.SetRuleMode(i, "maybe"); err == nil {
		t.Error("bad mode accepted")
	}
	// A kind switched to regex with a bad value is caught by Save.
	_ = p.SetRuleKind(i, "keyword")
	_ = p.SetRuleValue(i, "(x")
	_ = p.SetRuleKind(i, "regex")
	if err := p.Validate(); err == nil {
		t.Error("Validate passed a bad regex")
	}
	p.RemoveRule(i)
	if len(p.Rules()) != n || p.Validate() != nil {
		t.Error("remove rule")
	}
	for id, w := range map[string]float64{"weak": 0.2, "normal": 0.5, "strong": 0.9} {
		if StrengthWeight(id) != w || StrengthID(w) != id {
			t.Errorf("strength %s", id)
		}
	}
}

func TestTypesPreview(t *testing.T) {
	p, _ := newTypes(t)
	pv := p.Preview()
	if pv.Invalid || pv.Path != "receipts/2025/telecom/2025-03-12_telekom_RE-2025-0042.pdf" {
		t.Fatalf("preview = %+v", pv)
	}
	p.SetPathTemplate("../x/{invoice}.pdf")
	if pv := p.Preview(); !pv.Invalid || !pv.Escapes {
		t.Errorf("escape = %+v", pv)
	}
	p.SetPathTemplate("a/{bogus}.pdf")
	if pv := p.Preview(); !pv.Invalid || pv.Escapes || pv.Message == "" {
		t.Errorf("unknown = %+v", pv)
	}
	p.SetPathTemplate("")
	if !p.Preview().Invalid {
		t.Error("empty template valid")
	}
	p.SetPathTemplate("a/{text:Project}_{invoice}.pdf")
	if pv := p.Preview(); pv.Invalid || pv.Path != "a/Project_RE-2025-0042.pdf" {
		t.Errorf("text = %+v", pv)
	}
	e := p.ExampleValues()
	e.Invoice = ""
	p.SetExample(e)
	if pv := p.Preview(); !pv.Invalid || len(pv.Missing) != 1 || pv.Missing[0] != "invoice" {
		t.Errorf("missing = %+v", pv)
	}
}

func TestTypesInsertFolder(t *testing.T) {
	p, _ := newTypes(t)
	root := p.Root()
	got, err := p.InsertFolder(filepath.Join(root, "finance", "bank"))
	if err != nil || got != "finance/bank/{date:2006-01-02}_telekom_{invoice}.pdf" {
		t.Errorf("got %q, %v", got, err)
	}
	p.SetPathTemplate("a/{date:2006/01}_x.pdf") // slash inside a placeholder
	if got, _ := p.InsertFolder(filepath.Join(root, "z")); got != "z/{date:2006/01}_x.pdf" {
		t.Errorf("got %q", got)
	}
	if got, _ := p.InsertFolder(root); got != "{date:2006/01}_x.pdf" {
		t.Errorf("root: %q", got)
	}
	if _, err := p.InsertFolder(filepath.Dir(root)); !errors.Is(err, ErrOutsideRoot) {
		t.Errorf("outside: %v", err)
	}
}

func TestLegendAndLists(t *testing.T) {
	p, _ := newTypes(t)
	l := p.Legend()
	if len(l) < 8 || l[0].Token != "2006" || l[0].Example != "2025" {
		t.Errorf("legend = %+v", l[:1])
	}
	if got := SplitList("a, b,,c "); len(got) != 3 || JoinList(got) != "a, b, c" {
		t.Errorf("list = %v", got)
	}
}
