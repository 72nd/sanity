package analysis

import (
	"math"
	"sort"
)

// knnK is the number of neighbours that vote.
const knnK = 3

// Corpus is a TF-IDF index of filed documents' texts with kNN scoring.
type Corpus struct {
	docs []corpusDoc
	df   map[string]int
}

type corpusDoc struct {
	typeID string
	tf     map[string]float64
}

// Add indexes one filed document.
func (c *Corpus) Add(typeID, text string) {
	if c.df == nil {
		c.df = map[string]int{}
	}
	tf := termFreq(text)
	for t := range tf {
		c.df[t]++
	}
	c.docs = append(c.docs, corpusDoc{typeID, tf})
}

// Len is the number of indexed documents.
func (c *Corpus) Len() int { return len(c.docs) }

func termFreq(text string) map[string]float64 {
	tf := map[string]float64{}
	for _, t := range tokenize(text) {
		if len(t) < 3 || isNumeric(t) {
			continue
		}
		tf[t]++
	}
	for t, n := range tf {
		tf[t] = 1 + math.Log(n)
	}
	return tf
}

func (c *Corpus) idf(t string) float64 {
	return math.Log(float64(len(c.docs)+1)/float64(c.df[t]+1)) + 1
}

func (c *Corpus) vec(tf map[string]float64) (map[string]float64, float64) {
	v := make(map[string]float64, len(tf))
	var norm float64
	for t, f := range tf {
		w := f * c.idf(t)
		v[t] = w
		norm += w * w
	}
	return v, math.Sqrt(norm)
}

// Score returns, per type among the k=3 nearest documents, the best cosine
// similarity weighted by the type's share of the neighbours' similarity.
func (c *Corpus) Score(text string) map[string]float64 {
	out := map[string]float64{}
	if len(c.docs) == 0 {
		return out
	}
	q, qn := c.vec(termFreq(text))
	if qn == 0 {
		return out
	}
	type nb struct {
		typeID string
		sim    float64
	}
	var nbs []nb
	for _, d := range c.docs {
		v, vn := c.vec(d.tf)
		if vn == 0 {
			continue
		}
		var dot float64
		for t, w := range q {
			dot += w * v[t]
		}
		nbs = append(nbs, nb{d.typeID, dot / (qn * vn)})
	}
	sort.SliceStable(nbs, func(i, j int) bool { return nbs[i].sim > nbs[j].sim })
	if len(nbs) > knnK {
		nbs = nbs[:knnK]
	}
	var total float64
	sum, best := map[string]float64{}, map[string]float64{}
	for _, n := range nbs {
		total += n.sim
		sum[n.typeID] += n.sim
		if n.sim > best[n.typeID] {
			best[n.typeID] = n.sim
		}
	}
	if total == 0 {
		return out
	}
	for id := range sum {
		out[id] = best[id] * sum[id] / total
	}
	return out
}
