package goinsta

import (
	"fmt"
	"strconv"
	"strings"
)

type Device struct {
	Manufacturer     string `json:"manufacturer"`
	Model            string `json:"model"`
	CodeName         string `json:"code_name"`
	AndroidVersion   int    `json:"android_version"`
	AndroidRelease   int    `json:"android_release"`
	ScreenDpi        string `json:"screen_dpi"`
	ScreenResolution string `json:"screen_resolution"`
	Chipset          string `json:"chipset"`
}

type School struct{}

type PicURLInfo struct {
	Height int    `json:"height"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
}

type ErrorN struct {
	Message   string `json:"message"`
	Endpoint  string `json:"endpoint"`
	Status    string `json:"status"`
	ErrorType string `json:"error_type"`
}

type Error503 struct {
	Message string
}

func (e Error503) Error() string {
	return e.Message
}

func (e ErrorN) Error() string {
	return fmt.Sprintf(
		"Error while calling %s, status code %s: %s (%s)",
		e.Endpoint, e.Status, e.Message, e.ErrorType,
	)
}

type Error400 struct {
	Status  string `json:"status"`
	Message string `json:"message"`

	ErrorType  string `json:"error_type"`
	ErrorTitle string `json:"error_title"`
	ErrorBody  string `json:"error_body"`

	Code int

	Endpoint string `json:"endpoint"`

	Payload struct {
		ClientContext string `json:"client_context"`
		Message       string `json:"message"`
	} `json:"payload"`

	DebugInfo struct {
		Message   string `json:"string"`
		Retriable bool   `json:"retriable"`
		Type      string `json:"type"`
	} `json:"debug_info"`
}

func (e Error400) Error() string {
	var msg string
	if e.Payload.Message != "" {
		msg = e.Payload.Message
	}
	if e.DebugInfo.Message != "" {
		msg = e.DebugInfo.Message
	}
	if e.Message != "" {
		if msg != "" {
			msg += "; " + e.Message
		} else {
			msg = e.Message
		}
	}

	if e.Code == 0 {
		e.Code = 400
	}
	return fmt.Sprintf("Request Status Code %d: %s, %s", e.Code, e.Status, msg)
}

func (e *Error400) GetMessage() string {
	if e.ErrorType != "" {
		return e.ErrorType
	}
	if e.Message != "" {
		return e.Message
	}
	return ""
}

type friendResp struct {
	Status     string     `json:"status"`
	Friendship Friendship `json:"friendship_status"`
}

type Location struct {
	insta *Instagram

	ID               Int64   `json:"pk"`
	Name             string  `json:"name"`
	Address          string  `json:"address"`
	City             string  `json:"city"`
	ShortName        string  `json:"short_name"`
	Lng              float64 `json:"lng"`
	Lat              float64 `json:"lat"`
	ExternalSource   string  `json:"external_source"`
	FacebookPlacesID Int64   `json:"facebook_places_id"`
}

type Friendship struct {
	Following       bool `json:"following"`
	FollowedBy      bool `json:"followed_by"`
	IncomingRequest bool `json:"incoming_request"`
	OutgoingRequest bool `json:"outgoing_request"`
	Muting          bool `json:"muting"`
	Blocking        bool `json:"blocking"`
	IsBestie        bool `json:"is_bestie"`
	IsBlockingReel  bool `json:"is_blocking_reel"`
	IsMutingReel    bool `json:"is_muting_reel"`
	IsPrivate       bool `json:"is_private"`
	IsRestricted    bool `json:"is_restricted"`
}

type Images struct {
	Versions []Candidate `json:"candidates"`
}

func (img Images) GetBest() string {
	best := ""
	var mh, mw int
	for _, v := range img.Versions {
		if v.Width > mw || v.Height > mh {
			best = v.URL
			mh, mw = v.Height, v.Width
		}
	}
	return best
}

type Candidate struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	URL          string `json:"url"`
	ScansProfile string `json:"scans_profile"`
}

type Tag struct {
	In []struct {
		User                  User        `json:"user"`
		Position              []float64   `json:"position"`
		StartTimeInVideoInSec interface{} `json:"start_time_in_video_in_sec"`
		DurationInVideoInSec  interface{} `json:"duration_in_video_in_sec"`
	} `json:"in"`
}

type Caption struct {
	// can be both string or int
	ID              interface{} `json:"pk"`
	UserID          Int64       `json:"user_id"`
	Text            string      `json:"text"`
	Type            int         `json:"type"`
	CreatedAt       Int64       `json:"created_at"`
	CreatedAtUtc    Int64       `json:"created_at_utc"`
	ContentType     string      `json:"content_type"`
	Status          string      `json:"status"`
	BitFlags        int         `json:"bit_flags"`
	User            User        `json:"user"`
	DidReportAsSpam bool        `json:"did_report_as_spam"`
	MediaID         Int64       `json:"media_id"`
	HasTranslation  bool        `json:"has_translation"`
}

type Mentions struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Z        Int64   `json:"z"`
	Width    float64 `json:"width"`
	Height   float64 `json:"height"`
	Rotation float64 `json:"rotation"`
	IsPinned int     `json:"is_pinned"`
	User     User    `json:"user"`
}

type Video struct {
	Type   int    `json:"type"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
	ID     string `json:"id"`
}

type Tray struct {
	Stories              []*Reel `json:"tray"`
	StoryRankingToken    string  `json:"story_ranking_token"`
	FaceFilterNuxVersion int     `json:"face_filter_nux_version"`
	HasNewNuxStory       bool    `json:"has_new_nux_story"`
	NuxElegible          bool    `json:"stories_viewer_gestures_nux_eligible"`
	StickerVersion       float64 `json:"sticker_version"`
	ReponseTS            float64 `json:"response_timestamp"`
	Status               string  `json:"status"`
	EmojiReactionsConfig struct {
		UfiType                        float64 `json:"ufi_type"`
		DeliveryType                   float64 `json:"delivery_type"`
		OverlaySkinTonePickerEnabled   bool    `json:"overlay_skin_tone_picker_enabled"`
		SwipeUpToShowReactions         bool    `json:"swipe_up_to_show_reactions"`
		ComposerNuxType                float64 `json:"composer_nux_type"`
		HideStoryViewCount             bool    `json:"hide_story_view_count"`
		ReactionTrayInteractivePanning bool    `json:"reaction_tray_interactive_panning_enabled"`
		PersistentSelfStoryBadge       bool    `json:"persistent_self_story_badge_enabled"`
		SelfstoryBadging               bool    `json:"self_story_badging_enabled"`
		ExitTestNux                    bool    `json:"exit_test_nux_enabled"`
	} `json:"emoji_reactions_config"`
}

func (tray *Tray) set(insta *Instagram) {
	for i := range tray.Stories {
		tray.Stories[i].setValues(insta)
	}
}

// Int64 decodes JSON numbers and numeric strings; the API mixes both for ids.
type Int64 int64

func (n *Int64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return err
		}
		v = int64(f)
	}
	*n = Int64(v)
	return nil
}

func (n Int64) String() string { return strconv.FormatInt(int64(n), 10) }
