//go:build live

package goinsta

import (
	"os"
	"testing"
)

// go test -tags live -run TestLiveTranslationsInsights -v . [REEL_ID=<pk>_<owner>]
func TestLiveTranslationsInsights(t *testing.T) {
	insta, err := ImportSession("login.session.json")
	if err != nil {
		t.Fatal(err)
	}
	defer insta.ExportSession("login.session.json")
	ctx := t.Context()

	s, err := insta.Account.TranslationSettings(ctx)
	if err != nil {
		t.Fatalf("TranslationSettings: %v", err)
	}
	c := s.Creation
	t.Logf("settings: subtitles=%s/%v text=%v creation=%+v", s.SubtitlesLanguage, s.SubtitlesEnabled, s.TextTranslations, c)

	err = insta.Account.SetTranslationSettings(ctx, TranslationCreationUpdate{
		VoiceTranslation: &c.VoiceTranslation, LipSync: &c.LipSync, ApprovalFlow: &c.ApprovalFlow,
		AcceptedConsent: &c.AcceptedConsent, SeenConsent: &c.SeenConsent,
	})
	t.Logf("SetTranslationSettings (unchanged): %v", err)
	if after, err := insta.Account.TranslationSettings(ctx); err == nil && after.Creation.VoiceTranslation != c.VoiceTranslation {
		t.Errorf("voice translation changed: %v -> %v", c.VoiceTranslation, after.Creation.VoiceTranslation)
	}

	posts := insta.Account.User().Posts()
	if err := posts.Next(ctx); err != nil || len(posts.Items) == 0 {
		t.Logf("no own post for insights: %v", err)
	} else {
		in, err := posts.Items[0].Insights(ctx)
		t.Logf("Insights %s: %+v err=%v", posts.Items[0].ID, in, err)
	}

	if id := os.Getenv("REEL_ID"); id != "" {
		it, err := insta.MediaInfo(ctx, id)
		if err != nil {
			t.Fatalf("MediaInfo %s: %v", id, err)
		}
		vt, err := it.VoiceTranslations(ctx)
		t.Logf("VoiceTranslations %s: %+v err=%v", id, vt, err)
	}
}
