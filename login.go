package goinsta

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// errNoBearer is applyLoginResponse's signal that a login response carried no
// token, so the caller can tell 2FA/checkpoint apart from success.
var errNoBearer = errors.New("login response carried no authorization token")

// twoFactorState bridges Login (password accepted, 2FA required) and
// LoginTwoFactor (code submitted).
type twoFactorState struct {
	context string       // two_step_verification_context from the send_login response
	restore func() error // restores the pre-login attestation, deferred to the code step
}

const (
	// The logged-out CAA flow runs on the auth host.
	bloksLoginURL        = "https://i.instagram.com/api/v1/bloks/async_action/com.bloks.www.bloks.caa.login.async.send_login_request/"
	processClientDataURL = "https://i.instagram.com/api/v1/bloks/async_action/com.bloks.www.bloks.caa.login.process_client_data_and_redirect/"
	createKeystoreURL    = "https://i.instagram.com/api/v1/attestation/create_android_keystore/"

	clientEndpointLogin   = "com.bloks.www.caa.login.login_homepage:com.bloks.www.caa.login.login_homepage"
	clientEndpointUnknown = "bloks_unknown_class:bloks_unknown"
)

// ErrPasswordKeyUnknown means no ig-set-password-encryption-* headers have been
// seen yet; make one request first (or import a session that carries them).
var ErrPasswordKeyUnknown = fmt.Errorf("password encryption key unknown; fetch it from a response first")

// loginFlow is the state the app builds before submitting credentials.
type loginFlow struct {
	waterfallID    string
	aac            string // CAA_ACCOUNT_ACCESS_CONTEXT, from process_client_data_and_redirect
	challengeNonce string // from create_android_keystore
	inputPrefix    string // Bloks screen-instance prefix of the text-input ids, e.g. "hhj08h"
}

// Login replays the app's logged-out password login: register a fresh zca key
// (challenge nonce), render the login context (aac), then send the credentials.
// The attestation headers in use before the call are restored afterwards.
//
// When the account has two-factor on, Login returns ErrTwoFactorRequired; call
// LoginTwoFactor with a TOTP code to finish. The logged-out attestation stays
// applied until then, so do not make other requests in between.
func (insta *Instagram) Login(ctx context.Context, username, password string) (err error) {
	insta.mu.Lock()
	stale := insta.tfa
	insta.tfa = nil
	insta.mu.Unlock()
	if stale != nil && stale.restore != nil {
		if err := stale.restore(); err != nil {
			return err
		}
	}
	lo, restore, err := insta.applyLoginAttestation()
	if err != nil {
		return err
	}
	restoreNow := true
	defer func() {
		if !restoreNow {
			return
		}
		if rerr := restore(); rerr != nil && err == nil {
			err = rerr
		}
		if err == nil {
			insta.notifySessionChange()
		}
	}()

	flow := &loginFlow{waterfallID: generateUUID(), inputPrefix: bloksInstanceID()}
	if flow.challengeNonce, err = insta.createKeystore(ctx, lo.keyHash); err != nil {
		return fmt.Errorf("create_android_keystore: %w", err)
	}
	if flow.aac, err = insta.fetchAAC(ctx, flow.waterfallID); err != nil {
		return fmt.Errorf("process_client_data_and_redirect: %w", err)
	}
	err = insta.sendLogin(ctx, username, password, flow)
	if errors.Is(err, ErrTwoFactorRequired) {
		// Hand the attestation restore to LoginTwoFactor, which runs the next
		// calls under the same logged-out key.
		insta.mu.Lock()
		if insta.tfa != nil {
			insta.tfa.restore = restore
			restoreNow = false
		}
		insta.mu.Unlock()
	}
	return err
}

// applyLoginAttestation swaps in a fresh never-uploaded zca key for the logged-out
// flow and returns a restore that puts the prior attestation back.
func (insta *Instagram) applyLoginAttestation() (lo *Attestation, restore func() error, err error) {
	insta.mu.Lock()
	registered, prevAtt, prevSigned := insta.att, insta.curAtt, insta.attSignedAt
	insta.mu.Unlock()
	prevZCA, prevUSDID := insta.header("X-Meta-Zca"), insta.header("X-Meta-Usdid")

	base := registered
	if base == nil {
		base = &Attestation{BatteryLevel: 85, BatteryState: "Charging"}
	}
	if lo, err = base.loggedOut(); err != nil {
		return nil, nil, err
	}
	if err = insta.applyAttestation(lo); err != nil {
		return nil, nil, err
	}
	restore = func() error {
		if registered != nil {
			if rerr := insta.applyAttestation(registered); rerr != nil {
				return fmt.Errorf("restore attestation: %w", rerr)
			}
			return nil
		}
		insta.restoreHeader("X-Meta-Zca", prevZCA)
		insta.restoreHeader("X-Meta-Usdid", prevUSDID)
		insta.mu.Lock()
		insta.curAtt, insta.attSignedAt = prevAtt, prevSigned
		insta.mu.Unlock()
		return nil
	}
	return lo, restore, nil
}

func (insta *Instagram) restoreHeader(k, v string) {
	if v == "" {
		insta.headerOptions.Delete(k)
		return
	}
	insta.headerOptions.Store(k, v)
}

// loggedOutRequest is the logged-out profile: www-claim 0, no x-fb-rmd, no
// bandwidth sample yet.
func (insta *Instagram) loggedOutRequest(endpoint, friendly, clientEndpoint string, query map[string]string) *reqOptions {
	return &reqOptions{
		Endpoint:      endpoint,
		IsPost:        true,
		LoggedOut:     true,
		Ignore429:     true, // surface the rate limit instead of sleeping 8 minutes
		FriendlyName:  "IgApi: " + friendly,
		Query:         query,
		IgnoreHeaders: []string{"X-Fb-Rmd"},
		ExtraHeaders: map[string]string{
			"X-Ig-Www-Claim":       "0",
			"X-Ig-Client-Endpoint": clientEndpoint,
			// No bandwidth sample exists yet on a logged-out cold start.
			"X-Ig-Bandwidth-Speed-Kbps":   "-1.000",
			"X-Ig-Bandwidth-Totalbytes-B": "0",
			"X-Ig-Bandwidth-Totaltime-Ms": "0",
		},
	}
}

func (insta *Instagram) createKeystore(ctx context.Context, keyHash string) (string, error) {
	body, _, err := insta.sendRequest(ctx, insta.loggedOutRequest(createKeystoreURL,
		"attestation/create_android_keystore/", clientEndpointUnknown,
		map[string]string{"app_scoped_device_id": insta.uuid, "key_hash": keyHash}))
	if err != nil {
		return "", withBody(err, body)
	}
	var r struct {
		ChallengeNonce string `json:"challenge_nonce"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.ChallengeNonce == "" {
		return "", fmt.Errorf("no challenge_nonce in %s", snippet(body))
	}
	return r.ChallengeNonce, nil
}

// fetchAAC renders the logged-out entry point, which mints the account-access
// context the login request must echo back.
func (insta *Instagram) fetchAAC(ctx context.Context, waterfallID string) (string, error) {
	params := map[string]any{
		"is_from_logged_out":                            false,
		"logged_out_user":                               "",
		"qpl_join_id":                                   nil,
		"family_device_id":                              insta.fID,
		"device_id":                                     insta.dID,
		"offline_experiment_group":                      "caa_iteration_v3_perf_ig_4",
		"waterfall_id":                                  waterfallID,
		"logout_source":                                 "",
		"show_internal_settings":                        false,
		"last_auto_login_time":                          0,
		"disable_auto_login":                            false,
		"qe_device_id":                                  insta.uuid,
		"use_auto_login_interstitial":                   true,
		"disable_recursive_auto_login_interstitial":     true,
		"auto_login_interstitial_experiment_group_name": "",
		"is_from_logged_in_switcher":                    false,
		"switcher_logged_in_uid":                        "",
		"account_list":                                  []any{},
		"blocked_uid":                                   []any{},
		"INTERNAL_INFRA_THEME":                          "THREE_C",
		"layered_homepage_experiment_group":             "Deploy: Not in Experiment",
		"launched_url":                                  "",
		"sim_phone_numbers":                             []any{},
		"is_from_registration_reminder":                 false,
	}
	form, err := insta.bloksForm(params)
	if err != nil {
		return "", err
	}
	body, _, err := insta.sendRequest(ctx, insta.loggedOutRequest(processClientDataURL,
		"bloks/async_action/com.bloks.www.bloks.caa.login.process_client_data_and_redirect/",
		clientEndpointUnknown, form))
	if err != nil {
		return "", withBody(err, body)
	}
	return extractAAC(body)
}

// The aac sits in a Bloks global-state declaration:
// {"key":"CAA_ACCOUNT_ACCESS_CONTEXT:aac",...,"initial_lispy":"\t(fhy \"<aac>\")"}.
var reAAC = regexp.MustCompile(`CAA_ACCOUNT_ACCESS_CONTEXT:aac".{0,200}?\(fhy "([A-Za-z0-9_-]{20,})"`)

func extractAAC(body []byte) (string, error) {
	m := reAAC.FindSubmatch(unescape(body))
	if m == nil {
		return "", fmt.Errorf("no aac in response: %s", snippet(body))
	}
	return string(m[1]), nil
}

// bloksForm wraps params in the async_action form the app posts.
func (insta *Instagram) bloksForm(params map[string]any) (map[string]string, error) {
	p, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	bk, err := json.Marshal(map[string]any{
		"bloks_version": insta.sess.BloksVersionID,
		"styles_id":     "instagram",
		"theme_params": []any{map[string]any{
			"value": []string{"three_neutral_gray"}, "design_system_name": "XMDS",
		}},
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"params":              string(p),
		"bk_client_context":   string(bk),
		"bloks_versioning_id": insta.sess.BloksVersionID,
	}, nil
}

// attestParams reports that the device could not produce a keystore attestation
// (-1013, as on Waydroid) for the server's challenge.
func attestParams(challengeNonce string) (string, error) {
	type entry struct {
		Version        int    `json:"version"`
		Type           string `json:"type"`
		Errors         []int  `json:"errors"`
		ChallengeNonce string `json:"challenge_nonce"`
		SignedNonce    string `json:"signed_nonce"`
		KeyHash        string `json:"key_hash"`
	}
	b, err := json.Marshal(map[string][]entry{"attestation": {{
		Version: 2, Type: "keystore", Errors: []int{-1013}, ChallengeNonce: challengeNonce,
	}}})
	return string(b), err
}

func withBody(err error, body []byte) error {
	if len(body) > 0 {
		return fmt.Errorf("%w (server said: %s)", err, snippet(body))
	}
	return err
}

func (insta *Instagram) sendLogin(ctx context.Context, username, password string, flow *loginFlow) error {
	insta.mu.Lock()
	pkB64, keyID := insta.pwPubKey, insta.pwKeyID
	insta.mu.Unlock()
	if pkB64 == "" || keyID == "" {
		return ErrPasswordKeyUnknown
	}
	pem, err := base64.StdEncoding.DecodeString(pkB64)
	if err != nil {
		return fmt.Errorf("password key: %w", err)
	}
	kid, err := strconv.Atoi(keyID)
	if err != nil {
		return fmt.Errorf("password key id %q: %w", keyID, err)
	}
	enc, err := encryptPassword(password, kid, pem, time.Now())
	if err != nil {
		return err
	}
	form, err := insta.bloksForm(insta.loginParams(username, enc, flow))
	if err != nil {
		return err
	}
	attest, err := attestParams(flow.challengeNonce)
	if err != nil {
		return err
	}

	o := insta.loggedOutRequest(bloksLoginURL,
		"bloks/async_action/com.bloks.www.bloks.caa.login.async.send_login_request/",
		clientEndpointLogin, form)
	o.ExtraHeaders["X-Ig-Attest-Params"] = attest
	o.ExtraHeaders["X-Bloks-Prism-Button-Version"] = "CONTROL"
	o.ExtraHeaders["X-Bloks-Prism-Colors-Enabled"] = "false"
	body, _, err := insta.sendRequest(ctx, o)
	if err != nil {
		return withBody(err, body)
	}
	err = insta.applyLoginResponse(body)
	if !errors.Is(err, errNoBearer) {
		return err
	}
	if context2fa, ok := parseTwoFactorContext(body); ok {
		insta.mu.Lock()
		insta.tfa = &twoFactorState{context: context2fa}
		insta.mu.Unlock()
		return ErrTwoFactorRequired
	}
	return classifyLoginFailure(body)
}

// The failed-login dialog's kind sits in the Bloks payload as
// (f6m 6 "error_info" "<kind>"), e.g. generic_error_dialog or redirect_two_fac.
var reLoginErrorInfo = regexp.MustCompile(`"error_info" "([a-z_]+)"`)

// classifyLoginFailure explains a send_login response that carried no bearer
// and no 2FA context. A wrong password logs a FIRST_PASSWORD_FAILURE-style
// recovery event, which unlike the dialog text does not depend on the locale.
func classifyLoginFailure(body []byte) error {
	flat := unescape(body)
	if bytes.Contains(flat, []byte("PASSWORD_FAILURE")) {
		return ErrBadPassword
	}
	kind := "unknown"
	if m := reLoginErrorInfo.FindSubmatch(flat); m != nil {
		kind = string(m[1])
	}
	if kind == "redirect_two_fac" {
		return fmt.Errorf("two-factor required but its context was not found in the response: %s", snippet(body))
	}
	return fmt.Errorf("login failed (%s): %s", kind, snippet(body))
}

const (
	methodPickerURL = "https://i.instagram.com/api/v1/bloks/async_action/com.bloks.www.two_step_verification.method_picker.navigation.async/"
	verifyCodeURL   = "https://i.instagram.com/api/v1/bloks/async_action/com.bloks.www.two_step_verification.verify_code.async/"

	clientEndpointMethodPicker = "IgCdsScreenNavigationLoggerModule:com.bloks.www.two_step_verification.method_picker"
	clientEndpointTOTP         = "IgCdsScreenNavigationLoggerModule:com.bloks.www.two_factor_login.enter_totp_code"
)

// parseTwoFactorContext finds two_step_verification_context in the send_login
// response. Bloks encodes the next screen's params as a key list followed by a
// value list:
//
//	(f4i (dkc "two_step_verification_context" "flow_source" …) (dkc "AWR9…" "two_factor_login" …))
func parseTwoFactorContext(body []byte) (string, bool) {
	flat := unescape(body)
	const key = `"two_step_verification_context"`
	k := bytes.Index(flat, []byte(key))
	if k < 0 {
		return "", false
	}
	start := bytes.LastIndex(flat[:k], []byte("(dkc "))
	if start < 0 {
		return "", false
	}
	keys, end := lispyList(flat, start+len("(dkc "))
	idx := -1
	for i, t := range keys {
		if t == key {
			idx = i
		}
	}
	rest := bytes.TrimLeft(flat[end:], " \t\n")
	if idx < 0 || !bytes.HasPrefix(rest, []byte("(dkc ")) {
		return "", false
	}
	vals, _ := lispyList(rest, len("(dkc "))
	if idx >= len(vals) {
		return "", false
	}
	v := vals[idx]
	if len(v) < 3 || v[0] != '"' || v[len(v)-1] != '"' {
		return "", false
	}
	return v[1 : len(v)-1], true
}

// lispyList reads the space-separated items of a Bloks list starting at i (just
// past its opening "(dkc "), keeping nested lists whole. It returns the items
// and the index just past the closing paren.
func lispyList(b []byte, i int) ([]string, int) {
	var items []string
	for i < len(b) {
		switch b[i] {
		case ')':
			return items, i + 1
		case ' ', '\t', '\n':
			i++
		case '"':
			j := bytes.IndexByte(b[i+1:], '"')
			if j < 0 {
				return items, len(b)
			}
			items = append(items, string(b[i:i+j+2]))
			i += j + 2
		case '(':
			depth, j := 0, i
			for ; j < len(b); j++ {
				if b[j] == '(' {
					depth++
				} else if b[j] == ')' {
					if depth--; depth == 0 {
						break
					}
				}
			}
			items = append(items, string(b[i:min(j+1, len(b))]))
			i = j + 1
		default:
			j := i
			for j < len(b) && b[j] != ' ' && b[j] != ')' {
				j++
			}
			items = append(items, string(b[i:j]))
			i = j
		}
	}
	return items, len(b)
}

// LoginTwoFactor finishes a login that returned ErrTwoFactorRequired with a TOTP
// code (see TOTP): it selects the TOTP method, then submits the code, whose
// response carries the bearer. The pending login is consumed either way, so a
// rejected code needs a fresh Login. The attestation Login swapped out is
// restored on return.
func (insta *Instagram) LoginTwoFactor(ctx context.Context, code string) (err error) {
	insta.mu.Lock()
	tfa := insta.tfa
	insta.tfa = nil
	insta.mu.Unlock()
	if tfa == nil {
		return errors.New("no pending two-factor login; call Login and check for ErrTwoFactorRequired first")
	}
	defer func() {
		if tfa.restore != nil {
			if rerr := tfa.restore(); rerr != nil && err == nil {
				err = rerr
			}
		}
		if err == nil {
			insta.notifySessionChange()
		}
	}()
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("empty two-factor code")
	}
	if _, err := insta.twoStepAction(ctx, methodPickerURL, clientEndpointMethodPicker, tfa.context,
		map[string]any{"selected_method": "totp"},
		map[string]any{"should_fallback_to_sms": 0},
	); err != nil {
		return fmt.Errorf("method_picker: %w", err)
	}
	body, err := insta.twoStepAction(ctx, verifyCodeURL, clientEndpointTOTP, tfa.context,
		map[string]any{
			"code":                   code,
			"auth_secure_device_id":  "",
			"block_store_machine_id": "",
			"should_trust_device":    0,
			"device_id":              insta.dID,
			"family_device_id":       insta.fID,
			"machine_id":             insta.machineID(),
		},
		map[string]any{"challenge": "totp"},
	)
	if err != nil {
		return fmt.Errorf("verify_code: %w", err)
	}
	if errors.Is(insta.applyLoginResponse(body), errNoBearer) {
		return fmt.Errorf("two-factor code rejected (wrong or expired?): %s", snippet(body))
	}
	return nil
}

func (insta *Instagram) machineID() string {
	if mid := insta.header("X-Mid"); mid != "" {
		return mid
	}
	return insta.sess.Mid
}

// twoStepAction posts a two_step_verification async action, adding the fields
// every step sends to the step-specific client and server params.
func (insta *Instagram) twoStepAction(ctx context.Context, url, endpoint, context2fa string, client, server map[string]any) ([]byte, error) {
	client["cloud_trust_token"] = nil
	client["network_bssid"] = nil
	server["INTERNAL__latency_qpl_marker_id"] = 36707139
	server["INTERNAL__latency_qpl_instance_id"] = insta.qplInstance()
	server["device_id"] = insta.dID
	server["spectra_reg_login_data"] = nil
	server["two_step_verification_context"] = context2fa
	server["flow_source"] = "two_factor_login"
	form, err := insta.bloksForm(map[string]any{"client_input_params": client, "server_params": server})
	if err != nil {
		return nil, err
	}
	o := insta.loggedOutRequest(url, "bloks/async_action/"+actionName(url), endpoint, form)
	o.ExtraHeaders["X-Bloks-Prism-Button-Version"] = "CONTROL"
	o.ExtraHeaders["X-Bloks-Prism-Colors-Enabled"] = "false"
	body, _, err := insta.sendRequest(ctx, o)
	if err != nil {
		return nil, withBody(err, body)
	}
	return body, nil
}

// actionName is the com.bloks.www.* segment of an async_action URL, for the
// X-Fb-Friendly-Name header.
func actionName(url string) string {
	s := strings.TrimSuffix(url, "/")
	return s[strings.LastIndexByte(s, '/')+1:]
}

// loginParams builds the send_login_request body. Many fields are static context
// the app sends; the ones that matter are the credential, contact point and device.
func (insta *Instagram) loginParams(username, encPassword string, flow *loginFlow) map[string]any {
	return map[string]any{
		"client_input_params": map[string]any{
			"password":                                encPassword,
			"contact_point":                           username,
			"device_id":                               insta.dID,
			"family_device_id":                        insta.fID,
			"machine_id":                              insta.sess.Mid,
			"aac":                                     flow.aac,
			"login_attempt_count":                     1,
			"try_num":                                 1,
			"event_flow":                              "login_manual",
			"event_step":                              "home_page",
			"password_contains_non_ascii":             "false",
			"si_device_param_network_info":            networkInfo(),
			"flash_call_permission_status":            map[string]any{"READ_PHONE_STATE": "DENIED", "READ_CALL_LOG": "DENIED", "ANSWER_PHONE_CALLS": "DENIED"},
			"lois_settings":                           map[string]any{"lois_token": ""},
			"openid_tokens":                           map[string]any{},
			"accounts_list":                           []any{},
			"aymh_accounts":                           []any{},
			"blocked_uids":                            []any{},
			"sim_phones":                              []any{},
			"sso_accounts_auth_data":                  []any{},
			"device_emails":                           []any{},
			"fb_ig_device_id":                         []any{},
			"network_bssid":                           nil,
			"ig_vetted_device_nonces":                 nil,
			"cloud_trust_token":                       nil,
			"sso_token_map_json_string":               "",
			"client_known_key_hash":                   "",
			"secure_family_device_id":                 "",
			"auth_secure_device_id":                   "",
			"block_store_machine_id":                  "",
			"encrypted_msisdn":                        "",
			"app_manager_id":                          "",
			"headers_infra_flow_id":                   "",
			"zero_balance_state":                      "",
			"has_whatsapp_installed":                  0,
			"has_granted_read_contacts_permissions":   0,
			"has_granted_read_phone_permissions":      0,
			"should_show_nested_nta_from_aymh":        0,
			"gms_incoming_call_retriever_eligibility": "not_eligible",
		},
		"server_params": map[string]any{
			"credential_type":                              "password",
			"device_id":                                    insta.dID,
			"family_device_id":                             insta.fID,
			"qe_device_id":                                 insta.uuid,
			"waterfall_id":                                 flow.waterfallID,
			"server_login_source":                          "login",
			"login_source":                                 "Login",
			"login_surface":                                "login_home",
			"login_entry_point":                            "logged_out",
			"login_credential_type":                        "none",
			"two_step_login_type":                          "one_step_login",
			"access_flow_version":                          "pre_mt_behavior",
			"ar_event_source":                              "login_home_page",
			"reg_flow_source":                              "login_home_native_integration_point",
			"left_nav_button_action":                       "NONE",
			"layered_homepage_experiment_group":            "Deploy: Not in Experiment",
			"offline_experiment_group":                     "caa_iteration_v3_perf_ig_4",
			"caller":                                       "gslr",
			"INTERNAL__latency_qpl_marker_id":              36707139,
			"INTERNAL__latency_qpl_instance_id":            insta.qplInstance(),
			"username_text_input_id":                       flow.inputPrefix + ":123",
			"password_text_input_id":                       flow.inputPrefix + ":124",
			"is_caa_perf_enabled":                          1,
			"is_from_logged_out":                           0,
			"is_platform_login":                            0,
			"is_from_logged_in_switcher":                   0,
			"is_from_aymh":                                 0,
			"is_from_landing_page":                         0,
			"is_from_empty_password":                       0,
			"is_from_msplit_fallback":                      0,
			"is_from_password_entry_page":                  0,
			"is_from_assistive_id":                         0,
			"should_trigger_override_login_2fa_action":     0,
			"should_trigger_override_login_success_action": 0,
		},
	}
}

// networkInfo mirrors the static si_device_param_network_info block the app sends.
func networkInfo() map[string]any {
	sub := map[string]any{"is_mobile_data_enabled": 0}
	for _, k := range []string{
		"network_type", "is_data_roaming", "is_esim", "is_gsm_roaming", "is_sim_sms_capable",
		"sim_carrier_id", "sim_carrier_id_name", "sim_state", "sim_operator", "sim_operator_name",
		"signal_strength", "group_id_level_1", "network_operator",
	} {
		sub[k] = nil
	}
	return map[string]any{
		"active_subscriptions_info": nil, "default_subscription_info": sub,
		"active_data_subscription_info": nil, "default_sms_subscription_info": nil,
		"is_airplane_mode": 0, "is_active_network_cellular": 0, "is_device_sms_capable": 0,
		"sim_count": 0, "is_wifi": 0,
	}
}

func snippet(b []byte) string {
	const max = 300
	if len(b) > max {
		b = b[:max]
	}
	return string(b)
}

var (
	reAuth  = regexp.MustCompile(`(?i)Bearer IGT:2:[A-Za-z0-9+/=_-]+`)
	reRur   = regexp.MustCompile(`(?i)"ig-set-ig-u-rur":\s*"([^"]*)"`)
	reClaim = regexp.MustCompile(`(?i)"x-ig-set-www-claim":\s*"([^"]*)"`)
	reDSID  = regexp.MustCompile(`(?i)"ig-set-ig-u-ds-user-id":\s*"?([0-9]+)"?`)
	reMid   = regexp.MustCompile(`(?i)"ig-set-x-mid":\s*"([^"]*)"`)
	rePwKey = regexp.MustCompile(`(?i)"ig-set-password-encryption-pub-key":\s*"([A-Za-z0-9+/=]+)"`)
	rePwKID = regexp.MustCompile(`(?i)"ig-set-password-encryption-key-id":\s*"?([0-9]+)"?`)
)

// applyLoginResponse pulls the session headers out of the Bloks payload, where
// they sit inside a deeply backslash-escaped JSON string. Stripping backslashes
// first makes the key/value pairs matchable.
func (insta *Instagram) applyLoginResponse(body []byte) error {
	flat := unescape(body)
	auth := reAuth.Find(flat)
	if auth == nil {
		return errNoBearer
	}
	insta.headerOptions.Store("Authorization", string(auth))
	store := func(re *regexp.Regexp, key string) {
		if m := re.FindSubmatch(flat); m != nil && len(m[1]) > 0 {
			insta.headerOptions.Store(key, string(m[1]))
		}
	}
	store(reRur, "Ig-U-Rur")
	store(reClaim, "X-Ig-Www-Claim")
	store(reMid, "X-Mid")
	if k, id := rePwKey.FindSubmatch(flat), rePwKID.FindSubmatch(flat); k != nil && id != nil {
		insta.mu.Lock()
		insta.pwPubKey, insta.pwKeyID = string(k[1]), string(id[1])
		insta.mu.Unlock()
	}
	if m := reDSID.FindSubmatch(flat); m != nil {
		if id, err := strconv.ParseInt(string(m[1]), 10, 64); err == nil {
			if insta.uid.Swap(id) != id {
				insta.fbid.Store(0)
			}
			insta.Account.ID = Int64(id)
		}
	}
	return nil
}

// qplInstance is a client latency-logging instance id; any positive int works.
func (insta *Instagram) qplInstance() int64 { return time.Now().UnixNano() % 1_000_000_000_000 }

// unescape drops every backslash, flattening Bloks payloads that nest JSON
// strings several levels deep so their key/value pairs become matchable.
func unescape(body []byte) []byte {
	flat := make([]byte, 0, len(body))
	for _, b := range body {
		if b != '\\' {
			flat = append(flat, b)
		}
	}
	return flat
}

// bloksInstanceID mimics the 6-char base36 prefix Bloks gives a rendered screen.
func bloksInstanceID() string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 6)
	for i := range b {
		b[i] = digits[rand.IntN(len(digits))]
	}
	return string(b)
}
