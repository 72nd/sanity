package presenter

import (
	"context"
	"image"
	"image/color"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/72nd/sanity/pdf"
)

// countEngine counts the calls that reach the PDF layer and renders a page of
// 595x842 points with a black block at fixed page fractions, at any scale.
type countEngine struct {
	pdf.Engine
	metas, renders atomic.Int64
	mu             sync.Mutex
	scales         []float64
}

func newCountEngine() *countEngine { return &countEngine{Engine: pdf.NewFake("one", "two")} }

func (c *countEngine) Meta(path string) (pdf.Meta, error) {
	c.metas.Add(1)
	return c.Engine.Meta(path)
}

func (c *countEngine) Render(path string, page int, scale float64) (image.Image, error) {
	c.renders.Add(1)
	c.mu.Lock()
	c.scales = append(c.scales, scale)
	c.mu.Unlock()
	w, h := int(math.Ceil(595*scale)), int(math.Ceil(842*scale))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	x0, x1 := int(0.20*float64(w)), int(0.70*float64(w))
	y0, y1 := int(0.10*float64(h)), int(0.60*float64(h))
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 0, 0, 0xff})
		}
	}
	return img, nil
}

func (c *countEngine) calls() int64 { return c.metas.Load() + c.renders.Load() }

func sortWith(t *testing.T, n int) (*Sort, *countEngine) {
	p, _ := newSort(t, n)
	e := newCountEngine()
	p.pdf = e
	return p, e
}

func TestPreviewCacheHitDoesNotCallEngine(t *testing.T) {
	p, e := sortWith(t, 2)
	p.Step(0)
	cur, _ := p.Current()
	img1, pages, err := p.Preview(cur.Path, 0)
	if err != nil || pages != 2 || img1 == nil {
		t.Fatalf("preview: %v %d %v", img1, pages, err)
	}
	before := e.calls()
	img2, pages2, err := p.Preview(cur.Path, 0)
	if err != nil || pages2 != 2 || img2 != img1 {
		t.Fatalf("second preview differs: %v", err)
	}
	if e.calls() != before {
		t.Fatalf("cache hit called the engine (%d -> %d calls)", before, e.calls())
	}
	var hits int
	for _, tm := range p.Timings() {
		if tm.Kind == TimingCacheHit {
			hits++
		}
	}
	if hits != 1 {
		t.Fatalf("cache-hit timings = %d, want 1", hits)
	}
}

func TestPrefetchRendersNextAndFirst(t *testing.T) {
	p, e := sortWith(t, 3)
	p.Step(0)
	p.Step(1) // current = b, so next = c and the filing target = a
	cur, _ := p.Current()
	if done := p.Prefetch(context.Background()); done == nil {
		t.Fatal("nothing prefetched")
	} else {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("prefetch hangs")
		}
	}
	before := e.calls()
	for _, it := range p.Items() {
		if it.ID == cur.ID {
			continue
		}
		if _, _, err := p.Preview(it.Path, 0); err != nil {
			t.Fatal(err)
		}
	}
	if e.calls() != before {
		t.Fatalf("prefetched previews rendered again (%d extra calls)", e.calls()-before)
	}
}

func TestPreviewSingleflight(t *testing.T) {
	p, e := sortWith(t, 1)
	p.Step(0)
	cur, _ := p.Current()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := p.Preview(cur.Path, 0); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := e.renders.Load(); n != 1 {
		t.Fatalf("%d renders for concurrent identical previews, want 1", n)
	}
}

func TestPreviewScalePolicy(t *testing.T) {
	p, _ := sortWith(t, 1)
	for _, z := range []float64{0.5, 1, 1.5} {
		if s := p.PreviewScale(z); s != BaseScale {
			t.Errorf("PreviewScale(%v) = %v, want %v", z, s, BaseScale)
		}
	}
	if s := p.PreviewScale(1.6); s != 2*BaseScale {
		t.Errorf("PreviewScale(1.6) = %v", s)
	}
	if s := p.PreviewScale(3); s != 6 {
		t.Errorf("PreviewScale(3) = %v", s)
	}
	if s := p.PreviewScale(16); s != MaxScale {
		t.Errorf("PreviewScale(16) = %v", s)
	}
}

// visibleOrigin is the top left corner of the visible part as a fraction of
// the cropped content, for a scroll offset in display pixels of a fit-width
// view of the given width at a zoom factor.
func visibleOrigin(img image.Image, fitW, zoom, offX, offY float64) (fx, fy float64) {
	b := img.Bounds()
	dispW := fitW * zoom
	dispH := dispW * float64(b.Dy()) / float64(b.Dx())
	return offX / dispW, offY / dispH
}

func TestPreviewAtSharperKeepsViewState(t *testing.T) {
	p, e := sortWith(t, 1)
	p.Step(0)
	cur, _ := p.Current()
	small, _, err := p.PreviewAt(cur.Path, 0, BaseScale)
	if err != nil {
		t.Fatal(err)
	}
	if small.Bounds().Dx() >= int(595*BaseScale)*3/4 {
		t.Fatalf("not cropped: %v", small.Bounds())
	}
	sharp, _, err := p.PreviewAt(cur.Path, 0, 6)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.CachedScale(cur.Path, 0); got != 6 {
		t.Fatalf("cached scale = %v, want 6 (the smaller image must be replaced)", got)
	}
	if e.renders.Load() != 2 {
		t.Fatalf("renders = %d, want 2", e.renders.Load())
	}
	// Same aspect ratio and the same part of the page.
	sb, hb := small.Bounds(), sharp.Bounds()
	ra, rb := float64(sb.Dx())/float64(sb.Dy()), float64(hb.Dx())/float64(hb.Dy())
	if math.Abs(ra-rb)/ra > 0.002 {
		t.Fatalf("aspect changed: %v -> %v", ra, rb)
	}
	if hb.Dx() < 2*sb.Dx() {
		t.Fatalf("sharp image is not larger: %v vs %v", hb, sb)
	}
	// A saved view (zoom 3, offset in display pixels at fit width 800) shows
	// the same origin fraction of the page in both images.
	const fitW, zoom, offX, offY = 800.0, 3.0, 420.0, 910.0
	ax, ay := visibleOrigin(small, fitW, zoom, offX, offY)
	bx, by := visibleOrigin(sharp, fitW, zoom, offX, offY)
	if math.Abs(ax-bx) > 0.002 || math.Abs(ay-by) > 0.002 {
		t.Fatalf("restored view moved: (%.4f,%.4f) vs (%.4f,%.4f)", ax, ay, bx, by)
	}
	// The content block sits at the same place inside both crops.
	var fracs []float64
	for _, img := range []image.Image{small, sharp} {
		b := img.Bounds()
		first := -1
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, _, _, _ := img.At(x, b.Min.Y+b.Dy()/4).RGBA(); r == 0 {
				first = x - b.Min.X
				break
			}
		}
		if first < 0 {
			t.Fatalf("no content in %v", b)
		}
		fracs = append(fracs, float64(first)/float64(b.Dx()))
	}
	if math.Abs(fracs[0]-fracs[1]) > 0.005 {
		t.Fatalf("content moved inside the crop: %.4f vs %.4f", fracs[0], fracs[1])
	}
	// A normal-size request after the sharp one is a hit and returns the sharp image.
	before := e.calls()
	again, _, _ := p.Preview(cur.Path, 0)
	if again != sharp || e.calls() != before {
		t.Fatal("base request after a sharp render must be served from the cache")
	}
}

func TestPreviewAtCapsPixelsAndRendersBaseFirst(t *testing.T) {
	p, e := sortWith(t, 1)
	p.Step(0)
	cur, _ := p.Current()
	if _, _, err := p.PreviewAt(cur.Path, 0, 50); err != nil { // uncached and far above the cap
		t.Fatal(err)
	}
	e.mu.Lock()
	scales := append([]float64(nil), e.scales...)
	e.mu.Unlock()
	if len(scales) != 2 || scales[0] != BaseScale {
		t.Fatalf("render scales = %v, want the base first", scales)
	}
	px := 595 * 842 * scales[1] * scales[1]
	if px > MaxPreviewPixels*1.01 {
		t.Fatalf("%.1f megapixels exceed the cap", px/1e6)
	}
	if scales[1] < 6 {
		t.Fatalf("capped scale too low: %v", scales[1])
	}
}

func TestPreviewCacheLimitAndFileChange(t *testing.T) {
	p, e := sortWith(t, 6)
	for _, it := range p.Items() {
		if _, _, err := p.Preview(it.Path, 0); err != nil {
			t.Fatal(err)
		}
	}
	p.pv.mu.Lock()
	n := len(p.pv.entries)
	p.pv.mu.Unlock()
	if n > previewMaxEntries {
		t.Fatalf("%d cached previews, max %d", n, previewMaxEntries)
	}
	// A changed file is rendered again.
	it := p.Items()[len(p.Items())-1]
	p.Preview(it.Path, 0)
	before := e.renders.Load()
	if err := touchGrow(it.Path); err != nil {
		t.Fatal(err)
	}
	p.Preview(it.Path, 0)
	if e.renders.Load() != before+1 {
		t.Fatal("changed file served from the cache")
	}
}

func touchGrow(path string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("more")
	return err
}
