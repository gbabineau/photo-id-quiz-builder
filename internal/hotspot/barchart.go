package hotspot

import (
	"errors"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gbabineau/photo-id-quiz-builder/internal/trip"
)

const weeksPerYear = 48

// Barchart is the weekly observation frequency of every species at one hotspot, as shown on the
// eBird bar chart page. Index 0 is the first week of January; each month has four weeks.
type Barchart struct {
	HotspotID string        `json:"hotspotId"`
	FetchedAt time.Time     `json:"fetchedAt"`
	Samples   []float64     `json:"samples"`
	Rows      []BarchartRow `json:"species"`
}

// BarchartRow is one species line of a bar chart.
type BarchartRow struct {
	CommonName string    `json:"comName"`
	Weekly     []float64 `json:"weekly"`
}

// ErrNoBarchartData means the page held no bar chart, such as a blocked request or an empty hotspot.
var ErrNoBarchartData = errors.New("no bar chart data in page")

// ParseBarchartPage extracts the tab-separated frequency table embedded in a bar chart page.
func ParseBarchartPage(page string) (Barchart, error) {
	var chart Barchart
	lines := strings.Split(strings.ReplaceAll(page, "\r\n", "\n"), "\n")

	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "Sample Size:") {
			start = i
			break
		}
	}
	if start < 0 {
		return chart, ErrNoBarchartData
	}
	samples, err := parseWeeks(strings.Fields(strings.TrimPrefix(lines[start], "Sample Size:")))
	if err != nil {
		return chart, fmt.Errorf("parse sample sizes: %w", err)
	}
	chart.Samples = samples

	for _, line := range lines[start+1:] {
		if strings.HasPrefix(strings.TrimSpace(line), "<") {
			break
		}
		cells := strings.Split(strings.TrimRight(line, "\t\r "), "\t")
		if len(cells) != weeksPerYear+1 {
			continue
		}
		weekly, err := parseWeeks(cells[1:])
		if err != nil {
			continue
		}
		chart.Rows = append(chart.Rows, BarchartRow{CommonName: html.UnescapeString(strings.TrimSpace(cells[0])), Weekly: weekly})
	}
	if len(chart.Rows) == 0 {
		return chart, ErrNoBarchartData
	}
	return chart, nil
}

func parseWeeks(cells []string) ([]float64, error) {
	if len(cells) != weeksPerYear {
		return nil, fmt.Errorf("expected %d weekly values, found %d", weeksPerYear, len(cells))
	}
	values := make([]float64, len(cells))
	for i, c := range cells {
		v, err := strconv.ParseFloat(strings.TrimSpace(c), 64)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	return values, nil
}

// isSpeciesName reports whether a bar chart row is a full species rather than a spuh, slash,
// hybrid, or form such as "coot sp.", "Anas sp.", "Gull sp. x Gull sp." or "Rock Pigeon (Feral Pigeon)".
func isSpeciesName(name string) bool {
	return name != "" && !strings.Contains(name, " sp.") && !strings.Contains(name, "/") &&
		!strings.Contains(name, " x ") && !strings.Contains(name, "(")
}

// Species returns each species' frequency, in percent of checklists, over the inclusive month
// range, weighting each week by its number of checklists. A start after the end wraps past December.
func (b Barchart) Species(startMonth, endMonth int) []trip.HotspotSpecies {
	if startMonth < 1 || startMonth > 12 || endMonth < 1 || endMonth > 12 {
		startMonth, endMonth = 1, 12
	}
	var weeks []int
	for m := startMonth; ; m = m%12 + 1 {
		for w := (m - 1) * 4; w < m*4; w++ {
			weeks = append(weeks, w)
		}
		if m == endMonth {
			break
		}
	}

	var total float64
	for _, w := range weeks {
		total += b.Samples[w]
	}
	if total == 0 {
		return nil
	}
	result := make([]trip.HotspotSpecies, 0, len(b.Rows))
	for _, row := range b.Rows {
		if !isSpeciesName(row.CommonName) {
			continue
		}
		var seen float64
		for _, w := range weeks {
			seen += row.Weekly[w] * b.Samples[w]
		}
		if freq := math.Round(seen/total*10000) / 100; freq > 0 {
			result = append(result, trip.HotspotSpecies{CommonName: row.CommonName, Frequency: freq})
		}
	}
	return result
}
