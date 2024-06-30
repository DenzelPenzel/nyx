package main

import (
	"math"
	"slices"
	"sort"
)

const (
	msFactor = 1000000
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
