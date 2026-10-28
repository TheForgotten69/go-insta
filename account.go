package goinsta

import (
	"encoding/json"
)

type Account struct {
	insta *Instagram

	ID                         Int64        `json:"pk"`
	Username                   string       `json:"username"`
	FullName                   string       `json:"full_name"`
	Biography                  string       `json:"biography"`
	ProfilePicURL              string       `json:"profile_pic_url"`
	Email                      string       `json:"email"`
	PhoneNumber                string       `json:"phone_number"`
	IsBusiness                 bool         `json:"is_business"`
	Gender                     int          `json:"gender"`
	ProfilePicID               string       `json:"profile_pic_id"`
	CanSeeOrganicInsights      bool         `json:"can_see_organic_insights"`
	ShowInsightsTerms          bool         `json:"show_insights_terms"`
	HasAnonymousProfilePicture bool         `json:"has_anonymous_profile_picture"`
	IsPrivate                  bool         `json:"is_private"`
	IsUnpublished              bool         `json:"is_unpublished"`
	AllowedCommenterType       string       `json:"allowed_commenter_type"`
	IsVerified                 bool         `json:"is_verified"`
	MediaCount                 int          `json:"media_count"`
	FollowerCount              int          `json:"follower_count"`
	FollowingCount             int          `json:"following_count"`
	GeoMediaCount              int          `json:"geo_media_count"`
	ExternalURL                string       `json:"external_url"`
	HasBiographyTranslation    bool         `json:"has_biography_translation"`
	ExternalLynxURL            string       `json:"external_lynx_url"`
	HdProfilePicURLInfo        PicURLInfo   `json:"hd_profile_pic_url_info"`
	HdProfilePicVersions       []PicURLInfo `json:"hd_profile_pic_versions"`
	UsertagsCount              int          `json:"usertags_count"`
	HasChaining                bool         `json:"has_chaining"`
	ReelAutoArchive            string       `json:"reel_auto_archive"`
	PublicEmail                string       `json:"public_email"`
	PublicPhoneNumber          string       `json:"public_phone_number"`
	PublicPhoneCountryCode     string       `json:"public_phone_country_code"`
	ContactPhoneNumber         string       `json:"contact_phone_number"`
	Byline                     string       `json:"byline"`
	SocialContext              string       `json:"social_context,omitempty"`
	SearchSocialContext        string       `json:"search_social_context,omitempty"`
	MutualFollowersCount       float64      `json:"mutual_followers_count"`
	LatestReelMedia            Int64        `json:"latest_reel_media,omitempty"`
	CityID                     Int64        `json:"city_id"`
	CityName                   string       `json:"city_name"`
	AddressStreet              string       `json:"address_street"`
	DirectMessaging            string       `json:"direct_messaging"`
	Latitude                   float64      `json:"latitude"`
	Longitude                  float64      `json:"longitude"`
	Category                   string       `json:"category"`
	BusinessContactMethod      string       `json:"business_contact_method"`
	IsCallToActionEnabled      bool         `json:"is_call_to_action_enabled"`
	FbPageCallToActionID       string       `json:"fb_page_call_to_action_id"`
	Zip                        string       `json:"zip"`
	AllowContactsSync          bool         `json:"allow_contacts_sync"`
	CanBoostPost               bool         `json:"can_boost_post"`
}

func (account *Account) Sync() error {
	insta := account.insta
	body, err := insta.sendSimpleRequest(urlUserInfo, account.ID)
	if err != nil {
		return err
	}
	var resp struct {
		User Account `json:"user"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	*account = resp.User
	account.insta = insta
	return nil
}

func (account *Account) User() *User {
	return account.insta.NewUser(int64(account.ID))
}

func (account *Account) Stories() (*StoryMedia, error) {
	return account.insta.fetchStories(int64(account.ID))
}

type ReelSettings struct {
	AllowStoryReshare              bool   `json:"allow_story_reshare"`
	MessagePrefs                   string `json:"message_prefs"`
	PersistStoriesToPrivateProfile bool   `json:"persist_stories_to_private_profile"`
	ReelAutoArchive                string `json:"reel_auto_archive"`
	SaveRecentStoriesCaptures      bool   `json:"save_recent_stories_captures"`
	SaveToCameraRoll               bool   `json:"save_to_camera_roll"`
	BlockedReelsCount              int    `json:"blocked_reels_count"`
}

func (account *Account) ReelSettings() (*ReelSettings, error) {
	body, err := account.insta.sendSimpleRequest(urlReelSettings)
	if err != nil {
		return nil, err
	}
	var s ReelSettings
	return &s, json.Unmarshal(body, &s)
}

func (account *Account) SetStoryAutoArchive(on bool) error {
	v := "off"
	if on {
		v = "on"
	}
	q, err := signedBody(map[string]string{
		"_uid":              account.ID.String(),
		"_uuid":             account.insta.uuid,
		"reel_auto_archive": v,
	})
	if err != nil {
		return err
	}
	_, _, err = account.insta.sendRequest(&reqOptions{Endpoint: urlSetReelSettings, IsPost: true, Query: q})
	return err
}

func (account *Account) RestrictedUsers() ([]*User, error) {
	body, err := account.insta.sendSimpleRequest(urlRestrictedUsers)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Users []*User `json:"users"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	for _, u := range resp.Users {
		u.insta = account.insta
	}
	return resp.Users, nil
}

// SuggestedUsers is the "Suggested for you" list (discover/ayml) on the own profile.
func (account *Account) SuggestedUsers() ([]*User, error) {
	body, _, err := account.insta.sendRequest(&reqOptions{
		Endpoint: urlDiscoverAYML,
		IsPost:   true,
		Query: map[string]string{
			"_uuid":                  account.insta.uuid,
			"module":                 "self_profile",
			"phone_id":               account.insta.fID,
			"profile_chaining_check": "true",
		},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		SuggestedUsers struct {
			Suggestions []struct {
				User *User `json:"user"`
			} `json:"suggestions"`
		} `json:"suggested_users"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	users := make([]*User, 0, len(resp.SuggestedUsers.Suggestions))
	for _, s := range resp.SuggestedUsers.Suggestions {
		if s.User != nil {
			users = append(users, s.User)
		}
	}
	return account.insta.usersFrom(users, entrySuggested), nil
}
