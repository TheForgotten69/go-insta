package goinsta

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
)

// Story insights units: each is one section of the app's story insights screen.
const (
	StoryUnitOverview        = "overviewv2"
	StoryUnitViews           = "view"
	StoryUnitInteractions    = "interactions"
	StoryUnitProfileActivity = "profile_activity"

	storyInsightsApp = "com.bloks.www.insights.unit.media.organic.story."
)

var storyInsightsUnits = []string{StoryUnitOverview, StoryUnitViews, StoryUnitInteractions, StoryUnitProfileActivity}

// StoryInsights are a story's metrics as its insights screen shows them. A nil
// count is a row the screen leaves out: no taps of that kind, or no data yet
// (the app's screen showed its empty state at 1-3 views, data at 5 viewers).
type StoryInsights struct {
	Navigation *int // Forward + Back + NextStory + Exited
	Forward    *int
	Back       *int
	NextStory  *int
	Exited     *int
	// Rows holds each unit's label/value rows as displayed, keyed by StoryUnit*;
	// "--" is a count Instagram does not have.
	Rows map[string]map[string]string
}

type storyUnitParams struct {
	MediaFBIDV2       string `json:"media_fbidv2"`
	IsElevatedSurface bool   `json:"is_elevated_surface"`
	Origin            string `json:"origin"`
}

// StoryInsights loads one of the account's own stories' insights screen. The
// typed counts read the English labels of the default en_US locale. Live, only
// Navigation and Exited have shown as rows; the Forward, Back and Next story
// labels come from the screen's info sheet.
func (item *Item) StoryInsights(ctx context.Context) (*StoryInsights, error) {
	if item.FBID == 0 {
		info, err := item.insta.MediaInfo(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		item.FBID = info.FBID
	}
	params := storyUnitParams{MediaFBIDV2: item.FBID.String(), Origin: "unknown"}
	rows := make([]map[string]string, len(storyInsightsUnits))
	errs := make([]error, len(storyInsightsUnits))
	var wg sync.WaitGroup
	for i, unit := range storyInsightsUnits {
		wg.Go(func() {
			var tree any
			tree, errs[i] = item.insta.bloksComponent(ctx, storyInsightsApp+unit, params)
			rows[i] = bloksRows(tree)
		})
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	in := &StoryInsights{Rows: make(map[string]map[string]string, len(rows))}
	for i, unit := range storyInsightsUnits {
		in.Rows[unit] = rows[i]
	}
	count := func(label string) *int { return rowCount(in.Rows[StoryUnitInteractions][label]) }
	in.Navigation, in.Forward, in.Back, in.NextStory, in.Exited =
		count("Navigation"), count("Forward"), count("Back"), count("Next story"), count("Exited")
	return in, nil
}

func rowCount(v string) *int {
	n, err := strconv.Atoi(strings.ReplaceAll(v, ",", ""))
	if err != nil {
		return nil
	}
	return &n
}
