package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

func makeSnippet(s string, n int) string {
	if n <= 0 {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func parseDateOnly(s string) (time.Time, error) {
	if len(s) != 10 {
		return time.Time{}, fmt.Errorf("bad date")
	}
	return time.ParseInLocation("2006-01-02", s, time.Local)
}

func normalizeRating(v int) int {
	if v < 1 || v > 5 {
		return 3
	}
	return v
}

var allowedBaseWeathers = []string{"sunny", "cloudy", "overcast", "light_rain", "storm", "snow", "other"}
var allowedAmbientWeathers = []string{"fog", "windy", "hot", "cold", "rainbow", "extreme"}

func normalizeBaseWeather(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	for _, allowed := range allowedBaseWeathers {
		if value == allowed {
			return value, true
		}
	}
	return "", false
}

func normalizeAmbientWeathers(values []string) ([]string, bool) {
	if len(values) == 0 {
		return []string{}, true
	}
	allowed := make(map[string]struct{}, len(allowedAmbientWeathers))
	for _, value := range allowedAmbientWeathers {
		allowed[value] = struct{}{}
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, false
		}
		if _, ok := allowed[value]; !ok {
			return nil, false
		}
		seen[value] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for _, value := range allowedAmbientWeathers {
		if _, ok := seen[value]; ok {
			out = append(out, value)
		}
	}
	return out, true
}

func encodeAmbientWeathers(values []string) string {
	normalized, ok := normalizeAmbientWeathers(values)
	if !ok {
		return "[]"
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeAmbientWeathers(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil {
		return []string{}
	}
	normalized, ok := normalizeAmbientWeathers(values)
	if !ok {
		return []string{}
	}
	return normalized
}

func getStringAny(m map[string]any, key string) (string, bool) {
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }
	for k, v := range m {
		if norm(k) != key {
			continue
		}
		vs, ok := v.(string)
		if !ok {
			return "", false
		}
		return strings.TrimSpace(vs), true
	}
	return "", false
}

func getStringSliceAny(m map[string]any, key string) []string {
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "")) }
	for k, v := range m {
		if norm(k) != key {
			continue
		}
		items, ok := v.([]any)
		if !ok {
			return nil
		}
		result := make([]string, 0, len(items))
		for _, item := range items {
			text, ok := item.(string)
			if !ok {
				return nil
			}
			result = append(result, strings.TrimSpace(text))
		}
		return result
	}
	return nil
}

func fallbackTitle(content, date string) string {
	rn := []rune(strings.TrimSpace(content))
	max := 16
	if len(rn) < max {
		max = len(rn)
	}
	if max > 0 {
		return string(rn[:max])
	}
	return date
}

func startOfWeekMonday(t time.Time) time.Time {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	weekday := int(day.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return day.AddDate(0, 0, -(weekday - 1))
}

func buildContributionWeeks(entries []*Entry, end time.Time) [][]DayCell {
	counts := map[string]int{}
	maxCount := 0
	for _, e := range entries {
		if e.Day == "" {
			continue
		}
		counts[e.Day] += e.EditCount
		if counts[e.Day] > maxCount {
			maxCount = counts[e.Day]
		}
	}
	start := startOfWeekMonday(end.AddDate(0, 0, -364))
	endDate := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	var weeks [][]DayCell
	for cursor := start; !cursor.After(endDate); cursor = cursor.AddDate(0, 0, 7) {
		week := make([]DayCell, 0, 7)
		for i := 0; i < 7; i++ {
			day := cursor.AddDate(0, 0, i)
			key := day.Format("2006-01-02")
			count := counts[key]
			level := 0
			if count > 0 && maxCount > 0 {
				level = int(math.Ceil(float64(count) / float64(maxCount) * 4))
				if level < 1 {
					level = 1
				} else if level > 4 {
					level = 4
				}
			}
			week = append(week, DayCell{Date: key, Count: count, Level: level})
		}
		weeks = append(weeks, week)
	}
	return weeks
}

func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
