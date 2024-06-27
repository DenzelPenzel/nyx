package main

import (
	"fmt"
	"math"
	"slices"
	"sort"
)

const (
	msFactor   = 1000000
	numBuckets = 100

	maxHeight = 50
)

type Stats struct {
	Avg float64
	Min float64
	Max float64
	P50 float64
	P75 float64
	P90 float64
	P95 float64
	P99 float64
}

func GetStats(data []int) Stats {
	if len(data) == 0 {
		return Stats{}
	}

	sort.Ints(data)
	minv, maxv := slices.Min(data), slices.Max(data)

	return Stats{
		Avg: avg(data) / msFactor,
		Min: float64(minv / msFactor),
		Max: float64(maxv / msFactor),
		P50: percent(data, 0.5) / msFactor,
		P75: percent(data, 0.75) / msFactor,
		P90: percent(data, 0.9) / msFactor,
		P95: percent(data, 0.95) / msFactor,
		P99: percent(data, 0.99) / msFactor,
	}
}

func PrintStats(data []int) {
	if len(data) == 0 {
		return
	}

	p99Idx := pIdx(len(data), 0.99)
	data = data[:p99Idx]

	buckets := make([]int, numBuckets)
	minv := data[0]
	maxv := data[len(data)-1]
	step := float64(maxv-minv) / numBuckets
	prevCutIdx := 0
	maxBucket := 0
	maxBucketIdx := 0

	for i := 0; i < numBuckets; i++ {
		cut := float64(minv) + step*float64(i+1)
		count := 0
		j := prevCutIdx

		for ; j < len(data) && float64(data[j]) < cut; j++ {
			count++
		}

		prevCutIdx = j
		buckets[i] = count

		if count > maxBucket {
			maxBucket = count
			maxBucketIdx = i
		}
	}

	topBucketFmt := fmt.Sprintf("%%%vd\n", maxBucketIdx+3)
	topPointerRow := make([]rune, numBuckets)

	for i := 1; i < numBuckets-1; i++ {
		topPointerRow[i] = ' '
	}

	topPointerRow[maxBucketIdx+1] = 'v'

	heightRatio := float64(maxHeight) / float64(maxBucket)
	for i := 0; i < len(buckets); i++ {
		buckets[i] = int(math.Min(float64(maxHeight), math.Ceil(float64(buckets[i])*heightRatio)))
	}

	var hist []rune
	for i := maxHeight; i >= 0; i-- {
		for j := 0; j < numBuckets; j++ {
			if i == 0 {
				hist = append(hist, '=')
			} else if buckets[j] == i {
				hist = append(hist, '|')
				buckets[j]--
			} else {
				hist = append(hist, ' ')
			}
		}

		hist = append(hist, '\n')
	}

	gmin := float64(minv) / msFactor
	gmax := float64(maxv) / msFactor
	gmid := gmin + (gmax-gmin)/2

	pointerRow := make([]rune, numBuckets)

	for i := 1; i < numBuckets-1; i++ {
		pointerRow[i] = ' '
	}

	pointerRow[0] = '^'
	pointerRow[len(pointerRow)/2] = '^'
	pointerRow[len(pointerRow)-1] = '^'

	fmt.Printf(topBucketFmt, maxBucket)
	fmt.Println(string(topPointerRow))
	fmt.Print(string(hist))
	fmt.Println(string(pointerRow))
	fmt.Printf("%.4fms                                       %.4fms                                     %.4fms\n", gmin, gmid, gmax)
}

func avg(data []int) float64 {
	r := float64(0)
	for _, d := range data {
		r += float64(d)
	}
	return r / float64(len(data))
}

func percent(data []int, p float64) float64 {
	idx := pIdx(len(data), p)
	return float64(data[idx])
}

func pIdx(datalen int, p float64) int {
	w := math.Ceil(float64(datalen) * p)
	return int(min(w, float64(datalen-1)))
}
