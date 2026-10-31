package goinsta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	docTranslationsUser             = "156105369516447605998684255875"
	docUpsertTranslationsCreation   = "35466444201443961438414953916"
	rootTranslationsUser            = "fetch__XDTUserDict"
	rootUpsertTranslationsCreation  = "xig_upsert_user_translations_creation_settings"
	friendlyTranslationsUser        = "TranslationsUserQuery"
	friendlyUpsertTranslationsInput = "UpsertUserTranslationsCreationSettings"
)

// TranslationSettings are the account's translation preferences: what it reads
// translated, and how its own Reels get AI voice translation.
type TranslationSettings struct {
	PreferredLanguages      PreferredLanguages          `json:"preferred_languages"`
	TranslateFrom           []TranslateFromLanguage     `json:"translate_from_preference"`
	TextTranslations        bool                        `json:"has_text_translations_consumption_enabled"`
	SubtitlesLanguage       string                      `json:"unified_subtitles_display_lang"`
	SubtitlesEnabled        bool                        `json:"unified_subtitles_enabled_pref"`
	Creation                TranslationCreationSettings `json:"translations_creation_settings"`
	ViewerPredictedLanguage string                      `json:"viewer_predicted_language"`
}

// PreferredLanguages are ISO 639-1 short codes ("en", "fr").
type PreferredLanguages struct {
	Selected   []string `json:"selected_lang_shortcodes_ordered"`
	Unselected []string `json:"unselected_lang_shortcodes"`
}

type TranslateFromLanguage struct {
	Language string `json:"language"`
	Enabled  bool   `json:"enabled"`
}

// TranslationCreationSettings are the account-wide AI voice translation
// choices for its own Reels.
type TranslationCreationSettings struct {
	VoiceTranslation  bool     `json:"voice_translation_opt_in"`
	LipSync           bool     `json:"lip_sync_opt_in"`
	ApprovalFlow      bool     `json:"approval_flow_opt_in"`
	AcceptedConsent   bool     `json:"has_accepted_voice_translations_consent_nux"`
	SeenConsent       bool     `json:"has_seen_voice_translations_consent_nux"`
	DisabledLanguages []string `json:"user_disabled_languages"`
}

type translationsUserVars struct {
	UsePreferredLanguageSetting bool   `json:"use_preferred_language_setting"`
	ShouldFetchCreationSettings bool   `json:"should_fetch_creation_settings"`
	UserID                      string `json:"user_id"`
}

// TranslationSettings reads the account's translation preferences.
func (account *Account) TranslationSettings(ctx context.Context) (*TranslationSettings, error) {
	body, err := account.insta.graphQL(ctx, gqlQuery{
		friendly: friendlyTranslationsUser,
		docID:    docTranslationsUser,
		root:     rootTranslationsUser,
	}, translationsUserVars{
		UsePreferredLanguageSetting: true,
		ShouldFetchCreationSettings: true,
		UserID:                      account.ID.String(),
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data struct {
			User *TranslationSettings `json:"fetch__XDTUserDict"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Data.User == nil {
		return nil, errors.New(rootTranslationsUser + ": no user in response")
	}
	return resp.Data.User, nil
}

// TranslationCreationUpdate changes the account's AI voice translation
// choices; a nil field is left as it is (the app sends it as null).
type TranslationCreationUpdate struct {
	VoiceTranslation *bool
	LipSync          *bool
	ApprovalFlow     *bool
	AcceptedConsent  *bool
	SeenConsent      *bool
}

type upsertTranslationsVars struct {
	Data upsertTranslationsData `json:"data"`
}

type upsertTranslationsData struct {
	ClientMutationID string                 `json:"client_mutation_id"`
	Args             upsertTranslationsArgs `json:"args"`
}

type upsertTranslationsArgs struct {
	VoiceTranslation *bool  `json:"voice_translation_opt_in"`
	SeenConsent      *bool  `json:"has_seen_voice_translations_consent_nux"`
	UserIGID         string `json:"user_igid"`
	LipSync          *bool  `json:"lip_sync_opt_in"`
	AcceptedConsent  *bool  `json:"has_accepted_voice_translations_consent_nux"`
	ApprovalFlow     *bool  `json:"approval_flow_opt_in"`
}

// SetTranslationSettings saves u, as the app's AI voice translation screen does.
func (account *Account) SetTranslationSettings(ctx context.Context, u TranslationCreationUpdate) error {
	id := generateUUID()
	body, err := account.insta.graphQL(ctx, gqlQuery{
		url:      graphQLWWWURL,
		friendly: friendlyUpsertTranslationsInput,
		docID:    docUpsertTranslationsCreation,
		root:     rootUpsertTranslationsCreation,
	}, upsertTranslationsVars{Data: upsertTranslationsData{
		ClientMutationID: id,
		Args: upsertTranslationsArgs{
			VoiceTranslation: u.VoiceTranslation,
			SeenConsent:      u.SeenConsent,
			UserIGID:         account.ID.String(),
			LipSync:          u.LipSync,
			AcceptedConsent:  u.AcceptedConsent,
			ApprovalFlow:     u.ApprovalFlow,
		},
	}})
	if err != nil {
		return err
	}
	var resp struct {
		Data map[string]*struct {
			ClientMutationID string `json:"client_mutation_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}
	if r := resp.Data[rootUpsertTranslationsCreation]; r == nil || r.ClientMutationID != id {
		return fmt.Errorf("%s: mutation not acknowledged", rootUpsertTranslationsCreation)
	}
	return nil
}

// VoiceTranslations is a Reel's AI voice translation state. Both lists were
// empty on every Reel seen so far, so their element format is unverified.
type VoiceTranslations struct {
	Languages []string          `json:"translated_langs_for_autodub"`
	Status    []json.RawMessage `json:"autodub_status"`
}

// VoiceTranslations reads the Reel's AI voice translation state from its media info.
func (item *Item) VoiceTranslations(ctx context.Context) (*VoiceTranslations, error) {
	var resp struct {
		Items []VoiceTranslations `json:"items"`
	}
	if err := item.insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlMediaInfo, item.ID)}, &resp); err != nil {
		return nil, err
	}
	if len(resp.Items) == 0 {
		return nil, ErrNoMedia
	}
	return &resp.Items[0], nil
}
