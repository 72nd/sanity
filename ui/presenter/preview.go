package presenter

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"sync"
	"time"

	"github.com/72nd/sanity/pdf"
)

// Preview rendering policy. Scale 1.0 is 72 dpi.
const (
	// BaseScale is the render scale of a freshly opened preview (about 150 dpi).
	BaseScale = 2.0
	// zoomRerender is the zoom above which the preview is re-rendered sharper.
	zoomRerender = 1.5
	// MaxScale caps the scale of a sharp re-render.
	MaxScale = 8.0
	// MaxPreviewPixels caps the pixels of the full rendered page (24 megapixels).
	MaxPreviewPixels = 24_000_000

	previewMaxEntries = 4
	previewMaxBytes   = 256 << 20
)

// PreviewScale is the render scale the preview should have at a zoom factor
// (1 = fit width). The fit view and light zoom use BaseScale, which already
// oversamples the fit view; above zoom 1.5 the page is rendered at twice the
// base scale at least, then follows the zoom (BaseScale*zoom), rounded up to
// half steps so small zoom changes do not render again. PreviewAt applies the
// pixel cap.
func (p *Sort) PreviewScale(zoom float64) float64 {
	if zoom <= zoomRerender {
		return BaseScale
	}
	s := math.Ceil(BaseScale*zoom*2) / 2
	return min(max(s, 2*BaseScale), MaxScale)
}

// CachedScale is the render scale of the cached preview of a page, or 0 when
// it is not cached. The view compares it with PreviewScale to decide whether a
// sharper render is due.
func (p *Sort) CachedScale(path string, page int) float64 {
	return p.pv.scaleOf(path, page)
}

// PreviewAt renders a page (0-based, clamped) at a scale and cuts the empty
// margin off. The cut is decided once per page at BaseScale and applied to
// every later scale as page fractions, so the cropped image keeps its aspect
// ratio and shows the same part of the page: a zoom factor and scroll offset
// (in display pixels of the fit-width view) stay valid when the image is
// replaced by a sharper one. A cached image at this scale or sharper is
// returned without touching the PDF engine; a successful sharper render
// replaces the cached smaller one. The scale is capped so the full page stays
// within MaxPreviewPixels. Call it from a goroutine; it may block.
func (p *Sort) PreviewAt(path string, page int, scale float64) (image.Image, int, error) {
	t0 := time.Now()
	img, pages, hit, err := p.pv.get(p.pdf, path, page, scale)
	if err == nil {
		kind := TimingRender
		if hit {
			kind = TimingCacheHit
		}
		p.recordTiming(Timing{Kind: kind, Name: path, D: time.Since(t0), Cached: hit})
	}
	return img, pages, err
}

// Prefetch renders, in the background, the documents that open next: the one
// after the current in queue order (what Skip opens) and the first one (what
// filing opens). It returns at once; the channel closes when the work is done
// (nil when there is nothing to do). Call it from the UI goroutine after the
// current preview arrived and after each filing.
func (p *Sort) Prefetch(ctx context.Context) <-chan struct{} {
	items := p.queue.Items()
	idx := -1
	for i, it := range items {
		if it.ID == p.current {
			idx = i
		}
	}
	var ids []string
	if len(items) > 1 && idx >= 0 {
		ids = append(ids, items[(idx+1)%len(items)].ID)
	}
	if len(items) > 0 && idx != 0 {
		ids = append(ids, items[0].ID)
	}
	type job struct {
		path  string
		page  int
		scale float64
	}
	var jobs []job
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] || id == p.current {
			continue
		}
		seen[id] = true
		it, ok := p.queue.Get(id)
		if !ok {
			continue
		}
		v := p.View(id)
		jobs = append(jobs, job{it.Path, v.Page, p.PreviewScale(v.Zoom)})
	}
	if len(jobs) == 0 {
		return nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, j := range jobs {
			if ctx.Err() != nil {
				return
			}
			p.PreviewAt(j.path, j.page, j.scale) // errors surface when the document is opened
		}
	}()
	return done
}

// forgetPreview drops everything cached of a file that was filed or trashed.
func (p *Sort) forgetPreview(path string) {
	p.pv.drop(path)
	pdf.EvictPath(p.pdf, path)
}

type previewKey struct {
	path string
	page int
}

type previewEntry struct {
	img          image.Image
	pages        int
	scale        float64
	fullW, fullH int        // pixels of the full page at scale
	crop         [4]float64 // x0, y0, x1, y1 of the cut as fractions of the page
	cropped      bool
	mtime        time.Time
	size         int64
	used         uint64
}

func (e *previewEntry) bytes() int { return e.fullW * e.fullH * 4 }

type flight struct{ done chan struct{} }

// previewCache is the in-memory LRU of rendered previews. It has its own lock
// because workers and the prefetch goroutine use it next to the UI goroutine.
type previewCache struct {
	mu       sync.Mutex
	entries  map[previewKey]*previewEntry
	inflight map[previewKey]*flight
	tick     uint64
}

func (c *previewCache) scaleOf(path string, page int) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[previewKey{path, page}]; e != nil {
		return e.scale
	}
	return 0
}

func (c *previewCache) drop(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.entries {
		if k.path == path {
			delete(c.entries, k)
		}
	}
}

// lookup returns the entry when it is still valid for the file on disk.
func (c *previewCache) lookup(k previewKey, st os.FileInfo) *previewEntry {
	e := c.entries[k]
	if e != nil && (!e.mtime.Equal(st.ModTime()) || e.size != st.Size()) {
		delete(c.entries, k)
		e = nil
	}
	return e
}

func (c *previewCache) get(eng pdf.Engine, path string, page int, scale float64) (img image.Image, pages int, hit bool, err error) {
	scale = max(scale, 0.1)
	st, err := os.Stat(path)
	if err != nil {
		c.drop(path)
		return nil, 0, false, err
	}
	// Page clamping needs the page count; the engine remembers Meta, and a
	// cached page of this file already knows it.
	for {
		k := previewKey{path, max(page, 0)}
		c.mu.Lock()
		if e := c.lookup(k, st); e != nil && e.scale >= scale*0.999 {
			c.tick++
			e.used = c.tick
			c.mu.Unlock()
			return e.img, e.pages, true, nil
		}
		if f := c.inflight[k]; f != nil { // someone renders this page: wait, then look again
			c.mu.Unlock()
			<-f.done
			continue
		}
		f := &flight{done: make(chan struct{})}
		if c.inflight == nil {
			c.inflight = map[previewKey]*flight{}
		}
		c.inflight[k] = f
		old := c.lookup(k, st)
		c.mu.Unlock()

		img, pages, err = c.render(eng, k, scale, old, st)

		c.mu.Lock()
		delete(c.inflight, k)
		close(f.done)
		c.mu.Unlock()
		return img, pages, false, err
	}
}

// render makes the entry for a page (or its sharper replacement) and stores it.
func (c *previewCache) render(eng pdf.Engine, k previewKey, scale float64, old *previewEntry, st os.FileInfo) (image.Image, int, error) {
	if old == nil && scale > BaseScale {
		// The cut is decided at BaseScale so every scale shares it.
		if _, _, _, err := c.renderEntry(eng, k, BaseScale, nil, st); err != nil {
			return nil, 0, err
		}
		c.mu.Lock()
		old = c.lookup(k, st)
		c.mu.Unlock()
		if old != nil && old.scale >= scale*0.999 {
			return old.img, old.pages, nil
		}
	}
	img, pages, _, err := c.renderEntry(eng, k, scale, old, st)
	return img, pages, err
}

func (c *previewCache) renderEntry(eng pdf.Engine, k previewKey, scale float64, old *previewEntry, st os.FileInfo) (image.Image, int, *previewEntry, error) {
	pages := 0
	if old != nil {
		pages = old.pages
	} else {
		meta, err := eng.Meta(k.path)
		if err != nil {
			return nil, 0, nil, err
		}
		pages = max(meta.Pages, 1)
	}
	page := min(max(k.page, 0), pages-1)
	if old != nil {
		// Cap the pixels with the page size known from the cached render.
		pageArea := float64(old.fullW) * float64(old.fullH) / (old.scale * old.scale)
		if pageArea > 0 {
			scale = min(scale, math.Sqrt(MaxPreviewPixels/pageArea))
		}
		if scale <= old.scale*1.001 {
			return old.img, pages, old, nil
		}
	}
	full, err := eng.Render(k.path, page, scale)
	if err != nil {
		return nil, pages, nil, err
	}
	fb := full.Bounds()
	e := &previewEntry{pages: pages, scale: scale, fullW: fb.Dx(), fullH: fb.Dy(), mtime: st.ModTime(), size: st.Size()}
	if old != nil && old.cropped {
		e.crop, e.cropped = old.crop, true
		e.img = cutFraction(full, e.crop)
	} else if old != nil {
		e.img = full
	} else {
		cut := pdf.CropWhite(full)
		e.img = cut
		if cb := cut.Bounds(); cb != fb {
			e.cropped = true
			e.crop = [4]float64{
				float64(cb.Min.X-fb.Min.X) / float64(fb.Dx()), float64(cb.Min.Y-fb.Min.Y) / float64(fb.Dy()),
				float64(cb.Max.X-fb.Min.X) / float64(fb.Dx()), float64(cb.Max.Y-fb.Min.Y) / float64(fb.Dy()),
			}
		}
	}
	c.mu.Lock()
	c.tick++
	e.used = c.tick
	if c.entries == nil {
		c.entries = map[previewKey]*previewEntry{}
	}
	c.entries[k] = e
	c.evictLocked(k)
	c.mu.Unlock()
	return e.img, pages, e, nil
}

// evictLocked drops least recently used entries beyond the count and byte
// budget; the entry just stored stays.
func (c *previewCache) evictLocked(keep previewKey) {
	for {
		total := 0
		var oldest previewKey
		var oldestE *previewEntry
		for k, e := range c.entries {
			total += e.bytes()
			if k != keep && (oldestE == nil || e.used < oldestE.used) {
				oldest, oldestE = k, e
			}
		}
		if (len(c.entries) <= previewMaxEntries && total <= previewMaxBytes) || oldestE == nil {
			return
		}
		delete(c.entries, oldest)
	}
}

// cutFraction cuts the page fractions out of an image of the full page.
func cutFraction(img image.Image, f [4]float64) image.Image {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	r := image.Rect(
		b.Min.X+int(math.Round(f[0]*w)), b.Min.Y+int(math.Round(f[1]*h)),
		b.Min.X+int(math.Round(f[2]*w)), b.Min.Y+int(math.Round(f[3]*h)),
	).Intersect(b)
	if s, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok && !r.Empty() {
		return s.SubImage(r)
	}
	return img
}

// TimingKind names what a Timing measured.
type TimingKind string

const (
	TimingOpen     TimingKind = "open→preview" // document opened until its preview was shown
	TimingRender   TimingKind = "render"       // PreviewAt rendered
	TimingCacheHit TimingKind = "cache-hit"    // PreviewAt served from memory
	TimingAnalysis TimingKind = "analysis"     // analysis of one document
)

// Timing is one measured duration.
type Timing struct {
	Kind   TimingKind
	Name   string // file path or id
	D      time.Duration
	Cached bool
}

// Timings returns what was measured so far, oldest first. With SANITY_TIMING=1
// every measurement is also printed to stderr.
func (p *Sort) Timings() []Timing {
	p.tm.mu.Lock()
	defer p.tm.mu.Unlock()
	return append([]Timing(nil), p.tm.list...)
}

type timings struct {
	mu       sync.Mutex
	list     []Timing
	openedAt map[string]time.Time
}

func (p *Sort) recordTiming(t Timing) {
	p.tm.mu.Lock()
	if len(p.tm.list) >= 500 {
		p.tm.list = p.tm.list[100:]
	}
	p.tm.list = append(p.tm.list, t)
	p.tm.mu.Unlock()
	if os.Getenv("SANITY_TIMING") == "1" {
		fmt.Fprintf(os.Stderr, "sanity: timing %-13s %8.1f ms cached=%v %s\n", t.Kind, float64(t.D.Microseconds())/1000, t.Cached, t.Name)
	}
}

func (p *Sort) markOpened(id string) {
	p.tm.mu.Lock()
	if p.tm.openedAt == nil {
		p.tm.openedAt = map[string]time.Time{}
	}
	p.tm.openedAt[id] = time.Now()
	p.tm.mu.Unlock()
}

// PreviewShown tells the presenter that the preview of a document is on
// screen; it records the time since Open. Call it from the UI goroutine.
func (p *Sort) PreviewShown(id string) {
	p.tm.mu.Lock()
	t0, ok := p.tm.openedAt[id]
	delete(p.tm.openedAt, id)
	p.tm.mu.Unlock()
	if ok {
		name := id
		if it, found := p.queue.Get(id); found {
			name = it.Path
		}
		p.recordTiming(Timing{Kind: TimingOpen, Name: name, D: time.Since(t0)})
	}
}
