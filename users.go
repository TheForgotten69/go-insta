package goinsta

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"
)

type userResp struct {
	Status string `json:"status"`
	User   User   `json:"user"`
}

type User struct {
	insta       *Instagram
	entry       entryContext
	Collections *Collections

	ID                     Int64  `json:"pk"`
	Username               string `json:"username"`
	FullName               string `json:"full_name,omitempty"`
	Email                  string `json:"email,omitempty"`
	PhoneNumber            string `json:"phone_number,omitempty"`
	WhatsappNumber         string `json:"whatsapp_number,omitempty"`
	Gender                 int    `json:"gender,omitempty"`
	PublicEmail            string `json:"public_email,omitempty"`
	PublicPhoneNumber      string `json:"public_phone_number,omitempty"`
	PublicPhoneCountryCode string `json:"public_phone_country_code,omitempty"`
	ContactPhoneNumber     string `json:"contact_phone_number,omitempty"`

	IsPrivate                  bool   `json:"is_private"`
	IsVerified                 bool   `json:"is_verified"`
	ExternalURL                string `json:"external_url,omitempty"`
	ExternalLynxURL            string `json:"external_lynx_url,omitempty"`
	FollowerCount              int    `json:"follower_count"`
	FollowingCount             int    `json:"following_count"`
	ProfilePicID               string `json:"profile_pic_id,omitempty"`
	ProfilePicURL              string `json:"profile_pic_url,omitempty"`
	HasAnonymousProfilePicture bool   `json:"has_anonymous_profile_picture"`
	Biography                  string `json:"biography,omitempty"`
	BiographyWithEntities      struct {
		RawText  string        `json:"raw_text"`
		Entities []interface{} `json:"entities"`
	} `json:"biography_with_entities"`
	BiographyProductMentions []interface{} `json:"biography_products_mentions"`

	IsNeedy                        bool          `json:"is_needy"`
	IsInterestAccount              bool          `json:"is_interest_account"`
	IsVideoCreator                 bool          `json:"is_video_creator"`
	IsBusiness                     bool          `json:"is_business"`
	BestiesCount                   int           `json:"besties_count"`
	ShowBestiesBadge               bool          `json:"show_besties_badge"`
	RecentlyBestiedByCount         int           `json:"recently_bestied_by_count"`
	AccountType                    int           `json:"account_type"`
	AccountBadges                  []interface{} `json:"account_badges,omitempty"`
	FbIdV2                         Int64         `json:"fbid_"`
	IsUnpublished                  bool          `json:"is_unpublished"`
	UserTagsCount                  int           `json:"usertags_count"`
	UserTagReviewEnabled           bool          `json:"usertag_review_enabled"`
	FollowingTagCount              int           `json:"following_tag_count"`
	MutualFollowersID              []Int64       `json:"profile_context_mutual_follow_ids,omitempty"`
	FollowFrictionType             int           `json:"follow_friction_type"`
	ProfileContext                 string        `json:"profile_context,omitempty"`
	HasBiographyTranslation        bool          `json:"has_biography_translation"`
	HasSavedItems                  bool          `json:"has_saved_items"`
	HasChaining                    bool          `json:"has_chaining"`
	IsFavorite                     bool          `json:"is_favorite"`
	IsFavoriteForStories           bool          `json:"is_favorite_for_stories"`
	IsFavoriteForHighlights        bool          `json:"is_favorite_for_highlights"`
	IsProfileActionNeeded          bool          `json:"is_profile_action_needed"`
	CanBeReportedAsFraud           bool          `json:"can_be_reported_as_fraud"`
	CanSeeSupportInbox             bool          `json:"can_see_support_inbox"`
	CanSeeSupportInboxV1           bool          `json:"can_see_support_inbox_v1"`
	CanSeePrimaryCountryInsettings bool          `json:"can_see_primary_country_in_settings"`
	CanFollowHashtag               bool          `json:"can_follow_hashtag"`

	CanBoostPosts                  bool   `json:"can_boost_posts"`
	CanSeeOrganicInsights          bool   `json:"can_see_organic_insights"`
	CanConvertToBusiness           bool   `json:"can_convert_to_business"`
	CanCreateSponsorTags           bool   `json:"can_create_sponsor_tags"`
	CanCreateNewFundraiser         bool   `json:"can_create_new_standalone_fundraiser"`
	CanCreateNewPersonalFundraiser bool   `json:"can_create_new_standalone_personal_fundraiser"`
	CanBeTaggedAsSponsor           bool   `json:"can_be_tagged_as_sponsor"`
	PersonalAccountAdsPageName     string `json:"personal_account_ads_page_name,omitempty"`
	PersonalAccountAdsId           string `json:"personal_account_ads_page_id,omitempty"`
	Category                       string `json:"category,omitempty"`

	ShowShoppableFeed           bool `json:"show_shoppable_feed"`
	CanTagProductsFromMerchants bool `json:"can_tag_products_from_merchants"`

	IsMutedWordsGlobalEnabled bool   `json:"is_muted_words_global_enabled"`
	IsMutedWordsCustomEnabled bool   `json:"is_muted_words_custom_enabled"`
	AllowedCommenterType      string `json:"allowed_commenter_type,omitempty"`

	MediaCount          int   `json:"media_count"`
	IGTVCount           int   `json:"total_igtv_videos"`
	HasIGTVSeries       bool  `json:"has_igtv_series"`
	HasVideos           bool  `json:"has_videos"`
	TotalClipCount      int   `json:"total_clips_count"`
	TotalAREffects      int   `json:"total_ar_effects"`
	GeoMediaCount       int   `json:"geo_media_count"`
	HasProfileVideoFeed bool  `json:"has_profile_video_feed"`
	LiveBroadcastID     Int64 `json:"live_broadcast_id"`

	HasPlacedOrders bool `json:"has_placed_orders"`

	ShowInsightTerms           bool `json:"show_insights_terms"`
	ShowConversionEditEntry    bool `json:"show_conversion_edit_entry"`
	ShowPostsInsightEntryPoint bool `json:"show_post_insights_entry_point"`
	ShoppablePostsCount        int  `json:"shoppable_posts_count"`
	RequestContactEnabled      bool `json:"request_contact_enabled"`
	FeedPostReshareDisabled    bool `json:"feed_post_reshare_disabled"`
	CreatorShoppingInfo        struct {
		LinkedMerchantAccounts []interface{} `json:"linked_merchant_accounts,omitempty"`
	} `json:"creator_shopping_info,omitempty"`
	StandaloneFundraiserInfo struct {
		HasActiveFundraiser                 bool        `json:"has_active_fundraiser"`
		FundraiserId                        Int64       `json:"fundraiser_id"`
		FundraiserTitle                     string      `json:"fundraiser_title"`
		FundraiserType                      interface{} `json:"fundraiser_type"`
		FormattedGoalAmount                 string      `json:"formatted_goal_amount"`
		BeneficiaryUsername                 string      `json:"beneficiary_username"`
		FormattedFundraiserProgressInfoText string      `json:"formatted_fundraiser_progress_info_text"`
		PercentRaised                       interface{} `json:"percent_raised"`
	} `json:"standalone_fundraiser_info"`
	AggregatePromoteEngagement   bool         `json:"aggregate_promote_engagement"`
	AllowMentionSetting          string       `json:"allow_mention_setting,omitempty"`
	AllowTagSetting              string       `json:"allow_tag_setting,omitempty"`
	LimitedInteractionsEnabled   bool         `json:"limited_interactions_enabled"`
	ReelAutoArchive              string       `json:"reel_auto_archive,omitempty"`
	HasHighlightReels            bool         `json:"has_highlight_reels"`
	HightlightReshareDisabled    bool         `json:"highlight_reshare_disabled"`
	IsMemorialized               bool         `json:"is_memorialized"`
	HasGuides                    bool         `json:"has_guides"`
	HasAffiliateShop             bool         `json:"has_active_affiliate_shop"`
	CityID                       Int64        `json:"city_id"`
	CityName                     string       `json:"city_name,omitempty"`
	AddressStreet                string       `json:"address_street,omitempty"`
	DirectMessaging              string       `json:"direct_messaging,omitempty"`
	Latitude                     float64      `json:"latitude"`
	Longitude                    float64      `json:"longitude"`
	BusinessContactMethod        string       `json:"business_contact_method"`
	IncludeDirectBlacklistStatus bool         `json:"include_direct_blacklist_status"`
	HdProfilePicURLInfo          PicURLInfo   `json:"hd_profile_pic_url_info,omitempty"`
	HdProfilePicVersions         []PicURLInfo `json:"hd_profile_pic_versions,omitempty"`
	School                       School       `json:"school"`
	Byline                       string       `json:"byline"`
	SocialContext                string       `json:"social_context,omitempty"`
	SearchSocialContext          string       `json:"search_social_context,omitempty"`
	MutualFollowersCount         float64      `json:"mutual_followers_count"`
	LatestReelMedia              Int64        `json:"latest_reel_media,omitempty"`
	IsCallToActionEnabled        bool         `json:"is_call_to_action_enabled"`
	IsPotentialBusiness          bool         `json:"is_potential_business"`
	FbPageCallToActionID         string       `json:"fb_page_call_to_action_id,omitempty"`
	FbPayExperienceEnabled       bool         `json:"fbpay_experience_enabled"`
	Zip                          string       `json:"zip,omitempty"`
	Friendship                   Friendship   `json:"friendship_status"`
	AutoExpandChaining           bool         `json:"auto_expand_chaining"`

	AllowedToCreateNonprofitFundraisers        bool          `json:"is_allowed_to_create_standalone_nonprofit_fundraisers"`
	AllowedToCreatePersonalFundraisers         bool          `json:"is_allowed_to_create_standalone_personal_fundraisers"`
	IsElegibleToShowFbCrossSharingNux          bool          `json:"is_eligible_to_show_fb_cross_sharing_nux"`
	PageIdForNewSumaBizAccount                 interface{}   `json:"page_id_for_new_suma_biz_account"`
	ElegibleShoppingSignupEntrypoints          []interface{} `json:"eligible_shopping_signup_entrypoints"`
	IsIgdProductPickerEnabled                  bool          `json:"is_igd_product_picker_enabled"`
	IsElegibleForAffiliateShopOnboarding       bool          `json:"is_eligible_for_affiliate_shop_onboarding"`
	IsElegibleForSMBSupportFlow                bool          `json:"is_eligible_for_smb_support_flow"`
	ElegibleShoppingFormats                    []interface{} `json:"eligible_shopping_formats"`
	NeedsToAcceptShoppingSellerOnboardingTerms bool          `json:"needs_to_accept_shopping_seller_onboarding_terms"`
	SellerShoppableFeedType                    string        `json:"seller_shoppable_feed_type"`
	VisibleProducts                            int           `json:"num_visible_storefront_products"`
	IsShoppingCatalogSettingsEnabled           bool          `json:"is_shopping_settings_enabled"`
	IsShoppingCommunityContentEnabled          bool          `json:"is_shopping_community_content_enabled"`
	IsShoppingAutoHighlightEnabled             bool          `json:"is_shopping_auto_highlight_eligible"`
	IsShoppingCatalogSourceSelectionEnabled    bool          `json:"is_shopping_catalog_source_selection_enabled"`
	ProfessionalConversionSuggestedAccountType int           `json:"professional_conversion_suggested_account_type"`
	InteropMessagingUserfbid                   Int64         `json:"interop_messaging_user_fbid"`
	LinkedFbInfo                               struct{}      `json:"linked_fb_info"`
	HasElegibleWhatsappLinkingCategory         struct{}      `json:"has_eligible_whatsapp_linking_category"`
	ExistingUserAgeCollectionEnabled           bool          `json:"existing_user_age_collection_enabled"`
	AboutYourAccountBloksEntrypointEnabled     bool          `json:"about_your_account_bloks_entrypoint_enabled"`
	OpenExternalUrlWithInAppBrowser            bool          `json:"open_external_url_with_in_app_browser"`
	MerchantCheckoutStyle                      string        `json:"merchant_checkout_style"`

	ProfilePic []byte
}

func (user *User) SetInstagram(insta *Instagram) {
	user.insta = insta
}

func (insta *Instagram) NewUser(id int64) *User {
	return &User{insta: insta, ID: Int64(id)}
}

func (insta *Instagram) UserByID(id int64) (*User, error) {
	u := insta.NewUser(id)
	return u, u.Sync()
}

// Info uses the legacy users/{id}/info/ endpoint, which the app still calls for the own account.
func (user *User) Info() error {
	body, err := user.insta.sendSimpleRequest(urlUserInfo, user.ID)
	if err != nil {
		return err
	}
	return user.unmarshal(body)
}

func (user *User) Sync() error {
	body, _, err := user.insta.sendRequest(&reqOptions{
		Endpoint:     fmt.Sprintf(urlUserInfoStream, user.ID),
		FriendlyName: "IgApi: user_info_stream_by_id",
		IsPost:       true,
		Stream:       true,
		Priority:     "u=0",
		Query:        user.infoStreamQuery(),
		ExtraHeaders: map[string]string{"X-Ig-Accept-Hint": "image-grid"},
	})
	if err != nil {
		return err
	}
	return user.unmarshal(body)
}

func (user *User) infoStreamQuery() map[string]string {
	q := map[string]string{
		"_uuid":         user.insta.uuid,
		"push_disabled": "true",
	}
	switch {
	case int64(user.ID) == user.insta.userID():
		q["entry_point"], q["from_module"] = "self_profile", "self_profile"
	case user.entry != (entryContext{}):
		q["entry_point"], q["module"] = user.entry.entryPoint, user.entry.module
	default:
		q["entry_point"], q["module"] = "feed_story_header", "feed_timeline"
	}
	return q
}

func (user *User) unmarshal(body []byte) error {
	insta := user.insta
	resp := struct {
		User *User `json:"user"`
	}{User: user}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	user.insta = insta
	return nil
}

func (user *User) friendshipPayload(extra map[string]string) map[string]string {
	p := map[string]string{
		"user_id":                       user.ID.String(),
		"radio_type":                    "ethernet-none",
		"_uid":                          strconv.FormatInt(user.insta.userID(), 10),
		"_uuid":                         user.insta.uuid,
		"include_follow_friction_check": "1",
		"container_module":              "profile",
	}
	return MergeMapS(p, extra)
}

// Follow updates user.Friendship; for a private account it becomes an outgoing request.
func (user *User) Follow() error {
	return user.changeFriendship(urlUserFollow, map[string]string{"device_id": user.insta.dID})
}

func (user *User) Unfollow() error {
	return user.changeFriendship(urlUserUnfollow, nil)
}

func (user *User) changeFriendship(endpoint string, extra map[string]string) error {
	q, err := signedBody(user.friendshipPayload(extra))
	if err != nil {
		return err
	}
	body, _, err := user.insta.sendRequest(&reqOptions{
		Endpoint: fmt.Sprintf(endpoint, user.ID),
		Query:    q,
		IsPost:   true,
	})
	if err != nil {
		return err
	}
	var resp friendResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	user.Friendship = resp.Friendship
	return nil
}

func (user *User) GetFriendship() (*Friendship, error) {
	body, _, err := user.insta.sendRequest(&reqOptions{
		Endpoint: fmt.Sprintf(urlFriendship, user.ID),
		Query:    map[string]string{"is_external_deeplink_profile_view": "false"},
	})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &user.Friendship); err != nil {
		return nil, err
	}
	return &user.Friendship, nil
}

// SuggestedUsers is the "similar accounts" carousel shown on a profile.
func (user *User) SuggestedUsers() ([]*User, error) {
	body, _, err := user.insta.sendRequest(&reqOptions{
		Endpoint: urlDiscoverChaining,
		Query: map[string]string{
			"consumption_surface":      "profile_hscroll",
			"eligible_for_threads_cta": "false",
			"from_module":              "feed_timeline",
			"module":                   "profile",
			"profile_chaining_check":   "true",
			"request_entry_point":      "profile_auto_expand",
			"target_id":                user.ID.String(),
			"target_username":          user.Username,
		},
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Users []*User `json:"users"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return user.insta.usersFrom(resp.Users, entrySuggested), nil
}

func (user *User) DownloadProfilePic() ([]byte, error) {
	if user.ProfilePicURL == "" {
		return nil, ErrNoMedia
	}
	b, err := user.insta.download(user.ProfilePicURL)
	if err != nil {
		return nil, err
	}
	user.ProfilePic = b
	return b, nil
}

// DownloadProfilePicTo derives the file name and extension from the URL when dst is a directory.
func (user *User) DownloadProfilePicTo(dst string) error {
	folder, fn := path.Split(dst)
	b, err := user.DownloadProfilePic()
	if err != nil {
		return err
	}
	fn, err = getDownloadName(user.ProfilePicURL, fn)
	if err != nil {
		return err
	}
	return saveToFolder(folder, fn, b)
}
