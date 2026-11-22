package pdf

import "testing"

func TestFake(t *testing.T) {
	f := NewFake("one", "two")
	m, _ := f.Meta("x.pdf")
	if m.Pages != 2 {
		t.Fatalf("pages %d", m.Pages)
	}
	pt, err := f.Text("x.pdf", 1)
	if err != nil || pt.Text != "two" {
		t.Fatalf("%v %v", pt, err)
	}
	if _, err := f.Text("x.pdf", 2); err == nil {
		t.Fatal("want error")
	}
	img, err := f.Render("x.pdf", 0, 0.5)
	if err != nil || img.Bounds().Dx() != 297 {
		t.Fatalf("%v %v", img, err)
	}
}
