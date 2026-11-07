package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
	fbid := item.FBID
	if fbid == 0 {
		info, err := item.insta.MediaInfo(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		fbid = info.FBID
	}
	params := storyUnitParams{MediaFBIDV2: fbid.String(), Origin: "unknown"}
	in := &StoryInsights{Rows: map[string]map[string]string{}}
	for _, unit := range storyInsightsUnits {
		tree, err := item.insta.bloksComponent(ctx, storyInsightsApp+unit, params)
		if err != nil {
			return nil, err
		}
		in.Rows[unit] = bloksRows(tree)
	}
	nav := in.Rows[StoryUnitInteractions]
	in.Navigation, in.Forward, in.Back = rowCount(nav["Navigation"]), rowCount(nav["Forward"]), rowCount(nav["Back"])
	in.NextStory, in.Exited = rowCount(nav["Next story"]), rowCount(nav["Exited"])
	return in, nil
}

func rowCount(v string) *int {
	n, err := strconv.Atoi(strings.ReplaceAll(v, ",", ""))
	if err != nil {
		return nil
	}
	return &n
}

type bloksComponentVars struct {
	Params    bloksAppParams `json:"params"`
	BKContext bloksContext   `json:"bk_context"`
}

type bloksAppParams struct {
	Params            string           `json:"params"`
	BloksVersioningID string           `json:"bloks_versioning_id"`
	InfraParams       bloksInfraParams `json:"infra_params"`
	AppID             string           `json:"app_id"`
}

type bloksInfraParams struct {
	DeviceID string `json:"device_id"`
}

type bloksComponentParams struct {
	Params any `json:"params"`
}

type bloksContext struct {
	ThemeParams                   []bloksTheme `json:"theme_params"`
	JSCacheContents               []string     `json:"js_cache_contents"`
	IsFlipperEnabled              bool         `json:"is_flipper_enabled"`
	FBLiteUnityProperties         []string     `json:"fblite_unity_properties"`
	BlacklistedStyles             []string     `json:"blacklisted_styles"`
	Styles                        []string     `json:"styles"`
	AndroidDevicePerformanceClass int          `json:"android_device_performance_class"`
	AndroidOSAPILevel             int          `json:"android_os_api_level"`
}

func (insta *Instagram) bloksContext() bloksContext {
	return bloksContext{
		ThemeParams:           []bloksTheme{{Value: []string{"three_neutral_gray"}, DesignSystemName: "XMDS"}},
		JSCacheContents:       []string{},
		FBLiteUnityProperties: []string{},
		BlacklistedStyles:     []string{},
		Styles:                []string{},
		AndroidOSAPILevel:     insta.deviceInfo().AndroidVersion,
	}
}

// bloksComponent runs a com.bloks.www async component and returns its decoded payload.
func (insta *Instagram) bloksComponent(ctx context.Context, appID string, params any) (any, error) {
	p, err := json.Marshal(bloksComponentParams{Params: params})
	if err != nil {
		return nil, err
	}
	var root struct {
		Responses []struct {
			Initial struct {
				Payload string `json:"payload"`
			} `json:"initial_response"`
		} `json:"component_query_responses"`
	}
	err = insta.graphQLRoot(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyBloksAppRoot + appID,
		docID:    docBloksComponentQuery,
		root:     rootBloksComponentQuery,
		purpose:  "fetch",
		gzip:     true,
		priority: "u=1",
	}, bloksComponentVars{
		Params: bloksAppParams{
			Params:            string(p),
			BloksVersioningID: insta.sess.BloksVersionID,
			InfraParams:       bloksInfraParams{DeviceID: insta.uuid},
			AppID:             appID,
		},
		BKContext: insta.bloksContext(),
	}, &root)
	if err != nil {
		return nil, err
	}
	if len(root.Responses) == 0 || root.Responses[0].Initial.Payload == "" {
		return nil, fmt.Errorf("%s: %w", appID, ErrEmptyResponse)
	}
	var tree any
	return tree, json.Unmarshal([]byte(root.Responses[0].Initial.Payload), &tree)
}

// Bloks minifies component and attribute names per bloks version (bloksVerID):
// "㐈" is a box, "㐗" a text, and ")" a box's direction or a text's content.
const (
	bloksBox     = "㐈"
	bloksText    = "㐗"
	bloksContent = ")"
	bloksKids    = " "
)

// bloksRows collects the label/value rows of a Bloks tree: boxes laid out as a
// row of exactly two texts.
func bloksRows(tree any) map[string]string {
	rows := map[string]string{}
	var walk func(any)
	walk = func(n any) {
		switch n := n.(type) {
		case map[string]any:
			if box, ok := n[bloksBox].(map[string]any); ok && box[bloksContent] == "row" {
				if kids, ok := box[bloksKids].([]any); ok && len(kids) == 2 {
					label, okL := bloksTextOf(kids[0])
					value, okV := bloksTextOf(kids[1])
					if okL && okV {
						rows[label] = value
					}
				}
			}
			for _, v := range n {
				walk(v)
			}
		case []any:
			for _, v := range n {
				walk(v)
			}
		}
	}
	walk(tree)
	return rows
}

func bloksTextOf(n any) (string, bool) {
	m, _ := n.(map[string]any)
	t, _ := m[bloksText].(map[string]any)
	s, ok := t[bloksContent].(string)
	return s, ok
}
