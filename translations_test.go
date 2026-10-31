package goinsta

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/TheForgotten69/go-insta/internal/httplog"
)

func TestTranslationSettings(t *testing.T) {
	insta, r := newReplay(t, "translations_user")
	s, err := insta.Account.TranslationSettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	f := withGraphQLDeviations(fixture(t, "translations_user"))
	checkRequest(t, f, r.sent[0])
	checkVars(t, f, r.sent[0])
	if len(s.PreferredLanguages.Unselected) == 0 || len(s.TranslateFrom) == 0 || s.SubtitlesLanguage == "" {
		t.Errorf("settings not parsed: %+v", s)
	}
}

// The mutation echoes client_mutation_id, so this answers with the id the request sent.
func TestSetTranslationSettings(t *testing.T) {
	insta, _ := newReplay(t)
	f := withGraphQLDeviations(fixture(t, "translations_upsert"))
	var got sent
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		raw, _ := io.ReadAll(req.Body)
		got = sent{req: req, payload: payloadOf(req, raw)}
		var v upsertTranslationsVars
		_ = json.Unmarshal([]byte(got.payload["variables"].(string)), &v)
		return 200, fmt.Sprintf(`{"data":{"%s":{"client_mutation_id":%q}},"extensions":{"is_final":true}}`,
			rootUpsertTranslationsCreation, v.Data.ClientMutationID)
	})})
	on, off := true, false
	err := insta.Account.SetTranslationSettings(t.Context(), TranslationCreationUpdate{
		VoiceTranslation: &on, LipSync: &on, AcceptedConsent: &on, ApprovalFlow: &off,
	})
	if err != nil {
		t.Fatal(err)
	}
	checkRequest(t, f, got)
	var gotVars, wantVars map[string]any
	_ = json.Unmarshal([]byte(got.payload["variables"].(string)), &gotVars)
	_ = json.Unmarshal([]byte(f.ReqForm["variables"]), &wantVars)
	ga := gotVars["data"].(map[string]any)["args"].(map[string]any)
	wa := wantVars["data"].(map[string]any)["args"].(map[string]any)
	for k, w := range wa {
		if g, ok := ga[k]; !ok || g != w {
			t.Errorf("args.%s = %v, want %v", k, g, w)
		}
	}
	if len(ga) != len(wa) {
		t.Errorf("args keys = %d, want %d", len(ga), len(wa))
	}
}

func TestVoiceTranslations(t *testing.T) {
	insta, _ := newReplay(t)
	var path string
	insta.SetHTTPClient(&http.Client{Transport: handlerFunc(func(req *http.Request) (int, string) {
		path = req.URL.Path
		return 200, `{"items":[{"id":"1_2","translated_langs_for_autodub":["eng","spa"],"autodub_status":[]}],"status":"ok"}`
	})})
	vt, err := testItem(insta, "1_2").VoiceTranslations(t.Context())
	if err != nil || path != "/api/v1/media/1_2/info/" || strings.Join(vt.Languages, ",") != "eng,spa" {
		t.Errorf("VoiceTranslations = %+v, %v (path %s)", vt, err, path)
	}
}

// withGraphQLDeviations expects what graphQL() sends today where these captures
// differ: the app gzips per query (TranslationsUserQuery plain on graphql/query,
// the upsert gzipped on graphql_www) and sends the eager-dict null header on
// some queries. gqlQuery cannot express either yet.
func withGraphQLDeviations(f httplog.Entry) httplog.Entry {
	f.ReqHeaders = maps.Clone(f.ReqHeaders)
	if f.Path == "/graphql/query" {
		f.ReqHeaders["content-encoding"] = "gzip"
	} else {
		delete(f.ReqHeaders, "content-encoding")
	}
	if _, eager := f.ReqHeaders["x-ig-validate-null-in-legacy-eager-dict"]; eager {
		delete(f.ReqHeaders, "x-ig-validate-null-in-legacy-eager-dict")
		f.ReqHeaders["x-ig-validate-null-in-legacy-dict"] = "true"
	}
	return f
}

// checkVars compares the request's GraphQL variables with the fixture's.
func checkVars(t *testing.T, want httplog.Entry, got sent) {
	t.Helper()
	var g, w any
	_ = json.Unmarshal([]byte(fmt.Sprint(got.payload["variables"])), &g)
	_ = json.Unmarshal([]byte(want.ReqForm["variables"]), &w)
	if !reflect.DeepEqual(g, w) {
		t.Errorf("variables:\n got %v\nwant %v", g, w)
	}
}
