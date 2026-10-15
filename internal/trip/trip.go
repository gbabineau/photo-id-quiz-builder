// Package trip turns a trip itinerary into one runner-compatible quiz per day.
//
// It ports the legacy photo_id/process_trip.py pipeline: average hotspot
// frequencies, add species named in the itinerary, attach taxonomy, drop rare
// species, sort taxonomically, and keep each species only on the day where it
// is most likely.
package trip

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gbabineau/photo-id-quiz-builder/internal/quiz"
	"github.com/gbabineau/photo-id-quiz-builder/internal/taxonomy"
)

const (
	MentionedNote = "Mentioned in trip data"
	// minimumKeptFrequency is the percent below which a species is dropped, as in the legacy tool.
	minimumKeptFrequency = 1.0
	// Quizzes with more than splitThreshold species are split into parts of at most maxPartSize.
	splitThreshold = 40
	maxPartSize    = 30
)

// FlexString decodes either a JSON string or number into a string (legacy "day" values vary).
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("day must be a string or number")
	}
	*f = FlexString(n.String())
	return nil
}

type Hotspot struct {
	Name      string `json:"name"`
	HotspotID string `json:"hotspotId"`
}

type Day struct {
	Day          FlexString `json:"day"`
	FullDayTitle string     `json:"Full day title"`
	AMTitle      string     `json:"AM_title"`
	PMTitle      string     `json:"PM_title"`
	Hotspots     []Hotspot  `json:"hotspots"`
	Mentioned    []string   `json:"mentioned"`
}

type Trip struct {
	Name             string  `json:"name"`
	Location         string  `json:"location"`
	StartMonth       int     `json:"start_month"`
	EndMonth         int     `json:"end_month"`
	MinimumFrequency float64 `json:"minimum_frequency"`
	Itinerary        []Day   `json:"itinerary"`
}

// ReadTrip loads and validates a trip file.
func ReadTrip(path string) (Trip, error) {
	var t Trip
	data, err := os.ReadFile(path)
	if err != nil {
		return t, fmt.Errorf("read trip file: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return t, fmt.Errorf("trip file is not a JSON object: %w", err)
	}
	if _, ok := raw["itinerary"]; !ok {
		return t, errors.New("trip file has no \"itinerary\"")
	}
	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("parse trip file: %w", err)
	}
	return t, nil
}

// HotspotSpecies is one row of an eBird target list for a hotspot.
type HotspotSpecies struct {
	CommonName string  `json:"comName"`
	Frequency  float64 `json:"frequency"`
}

// ErrNoHotspotData means the source has no data for the hotspot.
var ErrNoHotspotData = errors.New("no hotspot data")

// HotspotSource supplies species frequencies (percent of checklists) for a hotspot ID such as
// "L485324" over the inclusive month range; a start after the end wraps past December.
type HotspotSource interface {
	Species(hotspotID string, startMonth, endMonth int) ([]HotspotSpecies, error)
}

// DirSource reads "<dir>/<hotspotID>.json" files, the layout of the legacy .cache/hotspots folder.
// Those files are not month-specific, so the month range is ignored.
type DirSource struct{ Dir string }

func (d DirSource) Species(hotspotID string, _, _ int) ([]HotspotSpecies, error) {
	data, err := os.ReadFile(filepath.Join(d.Dir, hotspotID+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoHotspotData
	}
	if err != nil {
		return nil, err
	}
	var list []HotspotSpecies
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse %s.json: %w", hotspotID, err)
	}
	return list, nil
}

var hotspotIDPattern = regexp.MustCompile(`L\d+`)

// HotspotID extracts the trailing eBird location code from an ID or hotspot URL.
func HotspotID(s string) string {
	s = strings.TrimSpace(s)
	if m := hotspotIDPattern.FindAllString(s, -1); len(m) > 0 {
		return m[len(m)-1]
	}
	return s
}

// DayQuiz is a generated quiz and the file name it should be saved under.
type DayQuiz struct {
	Day      string    `json:"day"`
	FileName string    `json:"fileName"`
	Quiz     quiz.Quiz `json:"quiz"`
}

// Result is the outcome of Build.
type Result struct {
	Quizzes  []DayQuiz `json:"quizzes"`
	Warnings []string  `json:"warnings"`
}

// Build generates one quiz per itinerary day.
func Build(t Trip, src HotspotSource, taxa []taxonomy.Taxon) Result {
	var result Result
	lookup := taxonLookup(taxa)
	days := make([][]quiz.Species, len(t.Itinerary))
	missing := map[string]bool{}

	for i, day := range t.Itinerary {
		label := dayLabel(day, i)
		species := hotspotSpecies(day, t, src, missing, &result.Warnings, label)
		species = addMentions(species, day.Mentioned, lookup, &result.Warnings, label)
		for k := range species {
			attachTaxonomy(&species[k], lookup)
		}
		species = keepFrequent(species)
		sort.SliceStable(species, func(a, b int) bool { return species[a].TaxonOrder < species[b].TaxonOrder })
		days[i] = species
	}
	removeSharedSpecies(days)

	used := map[string]bool{}
	for i, day := range t.Itinerary {
		q := quiz.Quiz{
			StartMonth: orDefault(t.StartMonth, 1),
			EndMonth:   orDefault(t.EndMonth, 12),
			Location:   t.Location,
			Notes:      dayNotes(day),
			Species:    days[i],
		}
		label := dayLabel(day, i)
		baseName := fmt.Sprintf("Day %s with species", sanitize(label))
		parts := quiz.Split(q, splitThreshold, maxPartSize)
		if len(parts) == 1 {
			name := fmt.Sprintf("%s 1-%d.json", baseName, len(q.Species))
			if len(q.Species) == 0 {
				name = fmt.Sprintf("Day %s with no species.json", sanitize(label))
			}
			result.Quizzes = append(result.Quizzes, DayQuiz{Day: label, FileName: uniqueName(name, used), Quiz: q})
			continue
		}
		first := 1
		for n, part := range parts {
			last := first + len(part.Species) - 1
			result.Quizzes = append(result.Quizzes, DayQuiz{
				Day:      fmt.Sprintf("%s (%d/%d)", label, n+1, len(parts)),
				FileName: uniqueName(fmt.Sprintf("%s %d-%d.json", baseName, first, last), used),
				Quiz:     part,
			})
			first = last + 1
		}
	}
	return result
}

func orDefault(v, d int) int {
	if v == 0 {
		return d
	}
	return v
}

func dayLabel(d Day, index int) string {
	if s := strings.TrimSpace(string(d.Day)); s != "" {
		return s
	}
	return strconv.Itoa(index + 1)
}

func dayNotes(d Day) string {
	if d.AMTitle != "" {
		return fmt.Sprintf("Morning: %s. Afternoon: %s", d.AMTitle, d.PMTitle)
	}
	return fmt.Sprintf("Full: %s.", d.FullDayTitle)
}

var unsafeFileChars = strings.NewReplacer("<", "-", ">", "-", ":", "-", "\"", "-", "/", "-", "\\", "-", "|", "-", "?", "-", "*", "-")

func sanitize(s string) string { return strings.TrimSpace(unsafeFileChars.Replace(s)) }

func uniqueName(name string, used map[string]bool) string {
	candidate := name
	base := strings.TrimSuffix(name, ".json")
	for n := 2; used[strings.ToLower(candidate)]; n++ {
		candidate = fmt.Sprintf("%s (%d).json", base, n)
	}
	used[strings.ToLower(candidate)] = true
	return candidate
}

// hotspotSpecies averages, per species, the frequencies from hotspots that meet the minimum.
func hotspotSpecies(day Day, t Trip, src HotspotSource, missing map[string]bool, warnings *[]string, label string) []quiz.Species {
	type acc struct {
		sum   float64
		count int
	}
	var order []string
	sums := map[string]*acc{}
	display := map[string]string{}

	for _, h := range day.Hotspots {
		id := HotspotID(h.HotspotID)
		list, err := src.Species(id, orDefault(t.StartMonth, 1), orDefault(t.EndMonth, 12))
		if err != nil {
			if !missing[id] {
				missing[id] = true
				*warnings = append(*warnings, fmt.Sprintf("Day %s: no hotspot data for %s (%s): %v", label, h.Name, id, err))
			}
			continue
		}
		for _, s := range list {
			if s.Frequency < t.MinimumFrequency {
				continue
			}
			key := nameKey(s.CommonName)
			if key == "" {
				continue
			}
			if sums[key] == nil {
				sums[key] = &acc{}
				order = append(order, key)
				display[key] = strings.TrimSpace(s.CommonName)
			}
			sums[key].sum += s.Frequency
			sums[key].count++
		}
	}

	result := make([]quiz.Species, 0, len(order))
	for _, key := range order {
		a := sums[key]
		avg := roundTo2(a.sum / float64(a.count))
		result = append(result, quiz.Species{CommonName: display[key], Frequency: &avg})
	}
	return result
}

func roundTo2(v float64) float64 {
	f, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'f', 2, 64), 64)
	return f
}

func nameKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func taxonLookup(taxa []taxonomy.Taxon) map[string]taxonomy.Taxon {
	m := make(map[string]taxonomy.Taxon, len(taxa))
	for _, t := range taxa {
		key := nameKey(t.CommonName)
		if key == "" {
			continue
		}
		prev, exists := m[key]
		if !exists || (prev.Category != "species" && t.Category == "species") {
			m[key] = t
		}
	}
	return m
}

func addMentions(species []quiz.Species, mentioned []string, lookup map[string]taxonomy.Taxon, warnings *[]string, label string) []quiz.Species {
	for _, name := range mentioned {
		key := nameKey(name)
		if _, ok := lookup[key]; !ok {
			*warnings = append(*warnings, fmt.Sprintf("Day %s: mentioned species %q not found in taxonomy", label, name))
			continue
		}
		found := false
		for i := range species {
			if nameKey(species[i].CommonName) == key {
				species[i].Notes = MentionedNote
				found = true
				break
			}
		}
		if !found {
			species = append(species, quiz.Species{CommonName: strings.TrimSpace(name), Notes: MentionedNote})
		}
	}
	return species
}

func attachTaxonomy(s *quiz.Species, lookup map[string]taxonomy.Taxon) {
	t, ok := lookup[nameKey(s.CommonName)]
	if !ok {
		return
	}
	s.CommonName = t.CommonName
	s.ScientificName = t.ScientificName
	s.SpeciesCode = t.SpeciesCode
	s.Category = t.Category
	s.TaxonOrder = t.TaxonOrder
	s.BandingCodes = t.BandingCodes
	s.ComNameCodes = t.ComNameCodes
	s.SciNameCodes = t.SciNameCodes
	s.Order = t.Order
	s.FamilyCode = t.FamilyCode
	s.FamilyComName = t.FamilyComName
	s.FamilySciName = t.FamilySciName
}

// keepFrequent drops species below 1%; species without a frequency (mentions) are kept.
func keepFrequent(species []quiz.Species) []quiz.Species {
	kept := species[:0]
	for _, s := range species {
		if s.Frequency == nil || *s.Frequency >= minimumKeptFrequency {
			kept = append(kept, s)
		}
	}
	return kept
}

// removeSharedSpecies keeps a species only on the day where it is best placed: a day
// that mentions it wins, otherwise the higher frequency wins, earlier day on ties.
func removeSharedSpecies(days [][]quiz.Species) {
	for i := range days {
		for j := i + 1; j < len(days); j++ {
			dropFromI, dropFromJ := map[string]bool{}, map[string]bool{}
			nextByName := map[string]quiz.Species{}
			for _, s := range days[j] {
				nextByName[nameKey(s.CommonName)] = s
			}
			for _, s := range days[i] {
				key := nameKey(s.CommonName)
				next, ok := nextByName[key]
				if !ok {
					continue
				}
				switch {
				case s.Notes != "":
					dropFromJ[key] = true
				case next.Notes != "":
					dropFromI[key] = true
				case freq(s) >= freq(next):
					dropFromJ[key] = true
				default:
					dropFromI[key] = true
				}
			}
			days[i] = without(days[i], dropFromI)
			days[j] = without(days[j], dropFromJ)
		}
	}
}

func freq(s quiz.Species) float64 {
	if s.Frequency == nil {
		return 0
	}
	return *s.Frequency
}

func without(species []quiz.Species, drop map[string]bool) []quiz.Species {
	if len(drop) == 0 {
		return species
	}
	kept := make([]quiz.Species, 0, len(species))
	for _, s := range species {
		if !drop[nameKey(s.CommonName)] {
			kept = append(kept, s)
		}
	}
	return kept
}
