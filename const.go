package goinsta

import "errors"

const (
	// The current build (captures from 2026-10-28) sends logged-in REST, GraphQL and
	// uploads to i.instagram.com; only cold-start calls still use b.i.instagram.com.
	baseURL    = "https://i.instagram.com/"
	graphQLURL = "https://i.instagram.com/graphql/query"
	apiURL     = "https://i.instagram.com/api/v1/"
	apiV2URL   = "https://i.instagram.com/api/v2/"

	// Values captured from Instagram for Android 453.0.0.55.79.
	appVersion     = "453.0.0.55.79"
	appVersionCode = "1100455565"
	bloksVerID     = "35d0a096ab0bb61bad7285862888e6bca4275b4b6cbd3572b5620c88a471642f"
	fbAnalytics    = "567067343352427"
	igCapabilities = "3brTv10C"
	connType       = "Ethernet"
	locale         = "en_US"

	// supportedCapabilities is the supported_capabilities_new value sent by
	// tray, story and highlight requests, verbatim from the capture.
	supportedCapabilities = `[{"name":"SUPPORTED_SDK_VERSIONS","value":"149.0,150.0,151.0,152.0,153.0,154.0,155.0,156.0,157.0,158.0,159.0,160.0,161.0,162.0,163.0,164.0,165.0,166.0,167.0,168.0,169.0,170.0,171.0,172.0,173.0,174.0,175.0,176.0,177.0,178.0,179.0,180.0,181.0,182.0,183.0,184.0,185.0,186.0,187.0,188.0,189.0,190.0,191.0,192.0,193.0,194.0,195.0,196.0,197.0,198.0,199.0,200.0,201.0,202.0"},{"name":"SUPPORTED_BETA_SDK_VERSIONS","value":"182.0-beta,183.0-beta,184.0-beta,185.0-beta,186.0-beta,187.0-beta,188.0-beta,189.0-beta,190.0-beta,191.0-beta,192.0-beta,193.0-beta,194.0-beta,195.0-beta,196.0-beta,197.0-beta,198.0-beta,199.0-beta,200.0-beta,201.0-beta,202.0-beta"},{"name":"FACE_TRACKER_VERSION","value":"14"},{"name":"segmentation","value":"segmentation_enabled"},{"name":"COMPRESSION","value":"ETC2_COMPRESSION"},{"name":"world_tracker","value":"world_tracker_enabled"}]`
)

var (
	Waydroid = Device{
		Manufacturer:     "Waydroid/waydroid",
		Model:            "WayDroid x86_64 Device",
		CodeName:         "waydroid_x86_64",
		AndroidVersion:   33,
		AndroidRelease:   13,
		ScreenDpi:        "243dpi",
		ScreenResolution: "3839x2160",
		Chipset:          "unknown",
	}
	timeOffset = getTimeOffset()
)

// Endpoints (with format vars), relative to /api/v1/ unless noted.
const (
	urlGetAccFamily = "multiple_accounts/get_account_family/"
	urlLogout       = "accounts/logout/"

	urlUserInfo         = "users/%d/info/"
	urlUserInfoStream   = "users/%d/info_stream/"
	urlUserFollow       = "friendships/create/%d/"
	urlUserUnfollow     = "friendships/destroy/%d/"
	urlFriendship       = "friendships/show/%d/"
	urlUserStories      = "feed/user/%d/story/"
	urlUserHighlights   = "highlights/%d/highlights_tray/"
	urlDiscoverChaining = "discover/chaining/"
	urlDiscoverAYML     = "discover/ayml/"
	urlRestrictedUsers  = "restrict_action/get_restricted_users/"
	urlReelSettings     = "users/reel_settings/"
	urlSetReelSettings  = "users/set_reel_settings/"

	urlTimeline         = "feed/timeline/"
	urlStories          = "feed/reels_tray/"
	urlReelsMediaStream = "feed/reels_media_stream/"
	urlMediaSeenV2      = "media/seen/" // api/v2
	urlStoryLike        = "story_interactions/send_story_like/"

	urlClipsDiscover  = "clips/discover/stream/"
	urlClipsWriteSeen = "clips/write_seen_state/"

	urlMediaLike           = "media/%s/like/"
	urlMediaSave           = "media/%s/save/"
	urlMediaDelete         = "media/%s/delete/"
	urlMediaOnlyMe         = "media/%s/only_me/"
	urlPinTimelineMedia    = "users/pin_timeline_media/"
	urlCommentAdd          = "media/%s/comment/"
	urlCommentLike         = "media/%s/comment_like/"
	urlCommentUnlike       = "media/%s/comment_unlike/"
	urlCommentBulkDelete   = "media/%s/comment/bulk_delete/"
	urlStreamComments      = "media/%s/stream_comments/"
	urlInlineChildComments = "media/%s/comments/%s/inline_child_comments/"

	urlAccountSerp        = "fbsearch/account_serp/"
	urlFriendshipShowMany = "friendships/show_many/"

	urlActivityRecent  = "news/inbox/"
	urlActivitySeen    = "news/inbox_seen/"
	urlCollectionsList = "collections/list/"
	urlCollectionFeed  = "feed/collection/%s/" // numbered collection; verified live
	urlSavedFeed       = "feed/saved/posts/"   // the all-media auto collection

	urlCreateNote = "media/create_note/v2/"

	urlUploadPhoto      = "rupload_igphoto/%s" // relative to baseURL
	urlConfigure        = "media/configure/"
	urlConfigureSidecar = "media/configure_sidecar/"
	urlConfigureStory   = "media/configure_to_story/"
	urlPDQHash          = "media/update_media_with_pdq_hash_info/"
)

var (
	ErrTooManyRequests   = errors.New("too many requests, please wait a few minutes before you try again")
	ErrLoggedOut         = errors.New("you have been logged out, capture a new session")
	ErrLoginRequired     = errors.New("session is not valid, capture a new session")
	ErrChallengeRequired = errors.New("challenge required, resolve it in the app and capture a new session")
	// ErrTwoFactorRequired means the password was accepted but the account has
	// two-factor on; finish with LoginTwoFactor.
	ErrTwoFactorRequired  = errors.New("two-factor required, finish with LoginTwoFactor")
	ErrCheckpointRequired = errors.New("checkpoint required, resolve it in the app and capture a new session")
	ErrBadPassword        = errors.New("the password you entered is incorrect")
	ErrSessionInvalid     = errors.New("session is missing authorization or user id")

	ErrInvalidImage       = errors.New("invalid file type, please use a jpeg or webp image")
	ErrCarouselMediaLimit = errors.New("an album takes 2 to 20 photos")

	ErrNoMore        = errors.New("no more posts available, page end has been reached")
	ErrMediaDeleted  = errors.New("sorry, this media has been deleted")
	ErrNoMedia       = errors.New("failed to download, no media found")
	ErrEmptyResponse = errors.New("empty response body")
)
