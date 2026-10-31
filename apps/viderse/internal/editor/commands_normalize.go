package editor

import (
	"math"
	"sort"
)

// These normalizations are performed by the engine items store after item.add
// and item.update. In particular sourceDuration is in whole source frames.
func fillNormalizeItem(item Object) {
	item["from"] = math.Max(0, fillRound(fillNum(item, "from", 0)))
	item["durationInFrames"] = math.Max(1, fillRound(fillNum(item, "durationInFrames", 1)))
	for _, key := range []string{"trimStart", "trimEnd", "sourceStart", "sourceEnd", "sourceDuration"} {
		if item[key] != nil {
			item[key] = math.Max(0, fillRound(fillNum(item, key, 0)))
		}
	}
	if item["sourceFps"] != nil {
		fps := fillNum(item, "sourceFps", 0)
		if fillFinite(fps) && fps > 0 {
			item["sourceFps"] = fillRound(fps*1000) / 1000
		} else {
			delete(item, "sourceFps")
		}
	}
	if (fillString(item, "type") == "video" || fillString(item, "type") == "audio") && item["sourceEnd"] != nil && item["sourceStart"] == nil {
		item["sourceStart"] = 0
	}
	if fillString(item, "type") == "shape" {
		for _, key := range []string{"taperStartWidth", "taperEndWidth", "taperStartLength", "taperEndLength"} {
			if item[key] != nil {
				maximum := 100.
				if key == "taperStartWidth" || key == "taperEndWidth" {
					maximum = 200
				}
				item[key] = math.Max(0, math.Min(maximum, fillNum(item, key, 0)))
			}
		}
		if fillBool(item, "strokeEnabled") && fillNum(item, "strokeWidth", 0) < 1 {
			item["strokeWidth"] = 1
		}
		if fillBool(item, "isMask") {
			item["blendMode"] = "normal"
		}
	}
}

// item.removeExact invokes the store's group-pruning write funnel. That funnel
// sorts tracks and derives the node session's default 100px track height.
func fillPruneTracks(tracks, items []Object) []Object {
	occupied := map[string]bool{}
	parents := map[string]string{}
	for _, track := range tracks {
		parents[fillString(track, "id")] = fillString(track, "parentTrackId")
	}
	for _, item := range items {
		occupied[fillString(item, "trackId")] = true
	}
	for id := range occupied {
		seen := map[string]bool{}
		for parent := parents[id]; parent != "" && !seen[parent]; parent = parents[parent] {
			occupied[parent] = true
			seen[parent] = true
		}
	}
	out := []Object{}
	for _, track := range tracks {
		if fillBool(track, "isGroup") && !occupied[fillString(track, "id")] {
			continue
		}
		track["height"] = 100
		if track["volume"] != nil {
			track["volume"] = math.Max(-60, math.Min(12, fillNum(track, "volume", 0)))
		}
		out = append(out, track)
	}
	sort.SliceStable(out, func(i, j int) bool { return fillNum(out[i], "order", 0) < fillNum(out[j], "order", 0) })
	return out
}
