package goinsta

import (
	"context"
	"fmt"
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

func (account *Account) Sync(ctx context.Context) error {
	insta := account.insta
	var resp struct {
		User Account `json:"user"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlUserInfo, account.ID)}, &resp); err != nil {
		return err
	}
	*account = resp.User
	account.insta = insta
	return nil
}

func (account *Account) User() *User {
	return account.insta.NewUser(int64(account.ID))
}

func (account *Account) Stories(ctx context.Context) (*Reel, error) {
	return account.insta.fetchStories(ctx, int64(account.ID))
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

func (account *Account) ReelSettings(ctx context.Context) (*ReelSettings, error) {
	var s ReelSettings
	if err := account.insta.callJSON(ctx, &reqOptions{Endpoint: urlReelSettings}, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (account *Account) SetStoryAutoArchive(ctx context.Context, on bool) error {
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
	_, err = account.insta.call(ctx, &reqOptions{Endpoint: urlSetReelSettings, IsPost: true, Query: q})
	return err
}

func (account *Account) RestrictedUsers(ctx context.Context) ([]*User, error) {
	var resp struct {
		Users []*User `json:"users"`
	}
	if err := account.insta.callJSON(ctx, &reqOptions{Endpoint: urlRestrictedUsers}, &resp); err != nil {
		return nil, err
	}
	return account.insta.usersFrom(resp.Users, entryContext{}), nil
}

// SuggestedUsers is the "Suggested for you" list (discover/ayml) on the own profile.
func (account *Account) SuggestedUsers(ctx context.Context) ([]*User, error) {
	var resp struct {
		SuggestedUsers struct {
			Suggestions []struct {
				User *User `json:"user"`
			} `json:"suggestions"`
		} `json:"suggested_users"`
	}
	err := account.insta.callJSON(ctx, &reqOptions{
		Endpoint: urlDiscoverAYML,
		IsPost:   true,
		Query: map[string]string{
			"_uuid":                  account.insta.uuid,
			"module":                 "self_profile",
			"phone_id":               account.insta.fID,
			"profile_chaining_check": "true",
		},
	}, &resp)
	if err != nil {
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
