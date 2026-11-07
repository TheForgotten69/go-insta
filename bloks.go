package goinsta

import (
	"context"
	"encoding/json"
	"fmt"
)

type bloksTheme struct {
	Value            []string `json:"value"`
	DesignSystemName string   `json:"design_system_name"`
}

var xmdsTheme = []bloksTheme{{Value: []string{"three_neutral_gray"}, DesignSystemName: "XMDS"}}

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
		ThemeParams:           xmdsTheme,
		JSCacheContents:       []string{},
		FBLiteUnityProperties: []string{},
		BlacklistedStyles:     []string{},
		Styles:                []string{},
		AndroidOSAPILevel:     insta.deviceInfo().AndroidVersion,
	}
}

// bloksComponent runs a com.bloks.www async component and decodes its payload.
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
	collectBloksRows(tree, rows)
	return rows
}

func collectBloksRows(n any, rows map[string]string) {
	switch n := n.(type) {
	case map[string]any:
		if label, value, ok := bloksRow(n); ok {
			rows[label] = value
		}
		for _, v := range n {
			collectBloksRows(v, rows)
		}
	case []any:
		for _, v := range n {
			collectBloksRows(v, rows)
		}
	}
}

func bloksRow(n map[string]any) (label, value string, ok bool) {
	box, _ := n[bloksBox].(map[string]any)
	kids, _ := box[bloksKids].([]any)
	if box[bloksContent] != "row" || len(kids) != 2 {
		return "", "", false
	}
	label, okL := bloksTextOf(kids[0])
	value, okV := bloksTextOf(kids[1])
	return label, value, okL && okV
}

func bloksTextOf(n any) (string, bool) {
	m, _ := n.(map[string]any)
	t, _ := m[bloksText].(map[string]any)
	s, ok := t[bloksContent].(string)
	return s, ok
}
