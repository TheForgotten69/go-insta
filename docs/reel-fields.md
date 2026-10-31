# Reel fields (configure_to_clips)

Every field the Instagram 453.0.0.55.79 app can send when it publishes a Reel, read from the
decompiled builders (`X.07EY`, the Reel builder, and `X.0CW8`, the builder shared by all media
types; obfuscated keys resolved through the app's string tables). Not every shared field applies
to Reels.

- **builder**: `reel` = Reel builder, `shared` = shared builder.
- **sent as**: `string` = a string value; `json` = raw JSON inside the signed body (the app's
  `AQi` setter); `upload/map` = a key of a nested map (upload params or another object).
- **go-insta**: the `ReelOptions` field that sets it, `(set)` when `UploadReel` always sends it,
  `Extra` otherwise (`ReelOptions.Extra` is applied last and can override anything).
- **verified**: observed live on the test account (a personal account) on 2026-10-31.

Nested keys (only inside another field's JSON): translation_input_language,
translation_lipsync_enabled, translation_approval_flow_enabled, translation_languages
(clips_voice_translation_metadata); scheduled_publish_time, scheduled_content_id, uuc_id
(content_scheduling_metadata); share_as_template_enabled (basel_template_user_setting);
comment_id (visual_replies_params); and editor metadata (asset_fbid, duration_ms, music_params,
name, offset_ms, original_audio_parts, original_media_id, remixed_original_sound_params,
start_time_ms, swappable_text, tool, has_voiceover_attribution).

| field | builder | sent as | go-insta | notes |
|---|---|---|---|---|
| `acr_browser_entry_point` | reel | string | Extra |  |
| `allow_multi_configures` | reel | string | Extra |  |
| `app_attribution_android_namespace` | shared | string | Extra |  |
| `app_attribution_id` | shared | string | Extra |  |
| `app_install_cta_info` | shared | string | Extra |  |
| `approve_translations_enabled` | shared | upload/map | Extra |  |
| `archive_only` | shared | string | Extra |  |
| `artifact_id` | shared | upload/map | Extra |  |
| `async_publish` | shared | string | Extra |  |
| `attributed_device_name` | shared | string | Extra |  |
| `attribution_content_url` | shared | string | Extra |  |
| `audience` | reel/shared | string | Audience | verified: besties echoed. Values: default, besties, followers_only, mutual_followers, only_me, fan_club, custom, friend_list, followers_only_rollcall, mutual_followers_rollcall, … |
| `audio_filter_infos` | reel | json/nested | Extra |  |
| `audio_muted` | shared | json | (set) | false |
| `autodub_creation_params` | shared | upload/map | — | upload params: {"autodub_enabled", "input_language", "output_languages":[{"language","is_lipsync"}], "approve_translations_enabled"}; on a single-request upload Instagram answers HTTP 400 "Single-write auth&init is not eligible…" |
| `autodub_enabled` | shared | upload/map | Extra |  |
| `basel_kit_id` | reel | string | Extra |  |
| `basel_video_composition_model` | reel | json | Extra |  |
| `bottom_camera_dial_selected` | shared | string | Extra |  |
| `branded_content_project_metadata` | shared | string | Extra |  |
| `broadcast_id` | shared | upload/map | Extra |  |
| `c2pa_metadata` | shared | upload/map | Extra |  |
| `camera_position` | shared | string | Extra |  |
| `camera_session_id` | reel | string | (set) |  |
| `camera_tools` | reel | json | Extra |  |
| `camera_upsell` | reel | string | Extra |  |
| `caption` | reel/shared | string | PostOptions.Caption | verified |
| `caption_add_on` | shared | string | Extra |  |
| `capture_type` | reel | string | Extra |  |
| `clips` | shared | json | (set) | [{"length", "source_type"}] |
| `clips_audio_metadata` | reel | json | Extra |  |
| `clips_creation_entry_point` | reel | string | Extra |  |
| `clips_segments_metadata` | reel | json | Extra |  |
| `clips_share_preview_to_feed` | reel | string | HideFromFeed (sent "1" unless set) | also show in the feed |
| `clips_spin_swappable_elements` | reel | string | Extra |  |
| `clips_transition_effects` | reel | json | Extra |  |
| `clips_voice_translation_metadata` | reel | string | VoiceTranslation | JSON string {"translation_input_language": ISO 639-3, "translation_lipsync_enabled": "1"/"0", "translation_approval_flow_enabled": "1"/"0", "translation_languages": [ISO 639-3]}; accepted, no visible effect on the test account |
| `contains_music_lyrics` | reel | string | Extra |  |
| `content_scheduling_metadata` | reel | string | ScheduledAt | JSON string {"scheduled_publish_time": unix seconds}; verified: stub media returned, Delete cancels |
| `content_tags` | shared | upload/map | Extra |  |
| `content_type` | shared | upload/map | Extra |  |
| `cover_frame_time_ms` | reel | string | CoverFrameMs | milliseconds into the video; not echoed |
| `cover_photo_cropping_type` | reel | string | Extra | "FOUR_BY_THREE" (always sent by the app) |
| `cover_photo_square_crop` | reel | json | Extra | raw JSON crop coordinates for the grid thumbnail |
| `cover_photo_type` | shared | upload/map | Extra |  |
| `create_pa_boost_post_access_token_not_expire` | shared | string | Extra |  |
| `created_from_highlight_info` | shared | string | Extra |  |
| `creator_geo_gating_info` | shared | string | Extra |  |
| `cross_app_share_fb_validation_check_bypass` | reel | json | Extra |  |
| `cross_app_share_type` | reel | string | Extra |  |
| `device` | shared | json | (set) |  |
| `direct_v2` | shared | upload/map | Extra |  |
| `disable_audio_filters` | reel | string | Extra |  |
| `disable_comments` | reel | string | DisableComments | verified: comments_disabled echoed true |
| `doodles_metadata` | reel | json | Extra |  |
| `draft_id` | reel | string | Extra |  |
| `edits` | shared | json | Extra |  |
| `effect_ids` | reel | json | Extra |  |
| `event_metadata` | reel | string | Extra |  |
| `extra` | shared | json | (set) | {"source_width","source_height"} |
| `extract_cover_frame` | shared | upload/map | Extra |  |
| `fan_club_configure_info` | reel | string | Extra |  |
| `fan_club_id` | reel | string | Extra |  |
| `for_album` | shared | upload/map | Extra |  |
| `for_direct_story` | shared | upload/map | Extra |  |
| `from_drafts` | shared | string | Extra |  |
| `funded_content_deal_id` | reel | string | Extra |  |
| `gen_ai_character_media_set_id` | reel | string | Extra |  |
| `genai_sharing_source_id` | shared | upload/map | Extra |  |
| `groups_destination_user_id` | reel | string | Extra |  |
| `height` | shared | upload/map | Extra |  |
| `hflip` | shared | upload/map | Extra |  |
| `hide_from_profile_grid` | shared | string | (set) | "false" |
| `hide_from_profile_reels_tab` | reel | string | Extra | only sent with linked_media_playlist_id |
| `horizon_world_id` | shared | string | Extra |  |
| `id` | shared | upload/map | Extra |  |
| `ig_timeline_metadata` | reel | json | Extra |  |
| `image_compression` | shared | upload/map | Extra |  |
| `input_kind` | shared | upload/map | Extra |  |
| `input_language` | shared | upload/map | Extra |  |
| `internal_features` | reel/shared | string | Extra |  |
| `iptc_metadata` | shared | upload/map | Extra |  |
| `is_affiliate_commission_eligible` | shared | string | Extra |  |
| `is_async_distribution` | shared | upload/map | Extra |  |
| `is_auto_resharing_to_story` | reel/shared | string | PostOptions.ShareToStory | verified: echoed true; Instagram reshares server-side |
| `is_basel` | shared | upload/map | Extra |  |
| `is_basel_clip` | shared | upload/map | Extra |  |
| `is_basel_overlay` | shared | upload/map | Extra |  |
| `is_clips_edited` | reel | string | Extra |  |
| `is_clips_video` | reel/shared | string/upload/map | Extra |  |
| `is_created_from_basel` | reel/shared | string | Extra |  |
| `is_created_with_contextual_music_recs` | reel | string | Extra |  |
| `is_created_with_sound_sync` | reel | string | Extra |  |
| `is_direct_voice` | shared | upload/map | Extra |  |
| `is_early_access` | reel | string | Extra |  |
| `is_feed_reshare` | shared | upload/map | Extra |  |
| `is_from_stories_midcard` | shared | string | Extra |  |
| `is_gifting_enabled` | reel | string | Gifting | test account: media_gifting_state not_applicable |
| `is_longform_video` | shared | upload/map | Extra |  |
| `is_media_kit_cover` | shared | upload/map | Extra |  |
| `is_optimistic_upload` | shared | upload/map | Extra |  |
| `is_pass_through_creation` | shared | string | Extra |  |
| `is_post_live_clips` | shared | upload/map | Extra |  |
| `is_quicksnap_recap` | shared | string | Extra |  |
| `is_reel` | reel | string | Extra |  |
| `is_shared_to_fb` | reel | string | Extra | "1"/"0" |
| `is_sidecar` | shared | upload/map | Extra |  |
| `is_spinnable` | reel | string | Extra |  |
| `is_story_image_with_music` | shared | upload/map | Extra |  |
| `is_story_interaction_response` | shared | upload/map | Extra |  |
| `is_template_disabled` | reel | string | DisableTemplate | "1"/"0" |
| `is_threads` | shared | upload/map | Extra |  |
| `is_upload_type_override_allowed` | shared | string | Extra |  |
| `is_wearable` | shared | upload/map | Extra |  |
| `length` | shared | json | (set) | seconds |
| `like_and_view_counts_disabled` | reel | string | PostOptions.HideLikeCounts | verified: echoed true |
| `linked_longform_media_info` | reel | string | Extra |  |
| `linked_media_info` | shared | string | Extra |  |
| `linked_media_playlist_id` | reel | string | Extra |  |
| `longform_clip_metadata` | reel | string | Extra |  |
| `mashup_info` | reel | json | Extra |  |
| `media_folder` | shared | string | Extra |  |
| `media_gating_info` | shared | string | Extra |  |
| `media_id` | reel/shared | nested/upload/map | Extra |  |
| `media_type` | shared | upload/map | Extra |  |
| `meta_gallery_media_metadata_id` | shared | string | Extra |  |
| `meta_gallery_media_source_item_key` | shared | string | Extra |  |
| `metagallery_media_id` | shared | string | Extra |  |
| `multi_sharing` | reel | string | Extra |  |
| `music_burnin_params` | shared | upload/map | Extra |  |
| `nav_chain` | shared | string | Extra |  |
| `ncs_share_to_reels_enabled` | shared | string | Extra |  |
| `oculus_attribution_exif` | shared | string | Extra |  |
| `organic_cta_type` | reel | string | Extra | call-to-action type |
| `origin` | shared | upload/map | Extra |  |
| `original_photo_pdq_hash` | shared | upload/map | Extra |  |
| `output_languages` | shared | upload/map | Extra |  |
| `overlay_data` | reel/shared | json | Extra |  |
| `pa_ncs_participant_id` | shared | string | Extra |  |
| `parent_template_clips_media_id` | reel | string | Extra |  |
| `photo_to_video_duration_ms` | shared | upload/map | Extra |  |
| `post_capture_editing_info` | reel | json | Extra |  |
| `post_id` | shared | upload/map | Extra |  |
| `poster_frame_index` | shared | json | (set) | 0 |
| `publish_id` | shared | string | Extra |  |
| `quick_snap_data` | shared | json | Extra |  |
| `quicksnap_recap_style` | shared | string | Extra |  |
| `ranking_info_token` | reel | string | Extra |  |
| `retry_context` | shared | upload/map | Extra |  |
| `reusable_template_assets_media_ids` | reel | json | Extra |  |
| `revshare_ads_toggled_on` | shared | string | Extra |  |
| `rotate` | shared | upload/map | Extra |  |
| `secret_reel_guessable` | reel | string | Extra |  |
| `secret_reel_hint` | reel | string | Extra |  |
| `segment_index` | shared | upload/map | Extra |  |
| `serial_number` | shared | string | Extra |  |
| `share_count_disabled` | reel | string | PostOptions.HideShareCounts | echoed |
| `share_to_facebook` | reel | string | Extra | with share_to_fb_destination_id, share_to_fb_destination_audience_type, cross_app_share_type |
| `share_to_fb_destination_audience_type` | reel | string | Extra |  |
| `share_to_fb_destination_id` | reel | string | Extra |  |
| `share_to_main_grid` | reel | string | Extra | only sent with linked_media_playlist_id (series); "0" for the hidden variant |
| `share_to_threads` | reel/shared | string | PostOptions.ShareToThreads | with share_to_threads_destination_id, _validation_bypass |
| `share_to_threads_destination_id` | reel/shared | string | PostOptions.ShareToThreads |  |
| `share_to_threads_validation_bypass` | reel/shared | json | PostOptions.ShareToThreads | JSON array |
| `share_type` | shared | upload/map | Extra |  |
| `shopping_data` | reel | string | Extra |  |
| `should_post_quietly` | shared | string | Extra | "1": no notification to followers |
| `smart_template_effect_id` | reel | string | Extra |  |
| `snippets_attribution_metadata` | reel | string | Extra |  |
| `source_type` | shared | string | (set) | "4" |
| `sponsor_tags` | shared | string | Extra |  |
| `stacked_timeline_metadata` | reel/shared | json | Extra |  |
| `stella_data` | shared | json/upload/map | Extra |  |
| `sticker_burnin_params` | shared | upload/map | Extra |  |
| `sticker_translations_enabled` | reel | string | Extra |  |
| `story_has_lyrics` | shared | upload/map | Extra |  |
| `supplemental_audio_tracks` | shared | upload/map | Extra |  |
| `tap_models` | reel | string | Extra |  |
| `template_clips_media_id` | reel | string | Extra |  |
| `template_opt_in_status` | reel | string | Extra |  |
| `text_post_share_to_ig_story_stickers` | reel | string | Extra |  |
| `third_party_downloads_enabled` | reel | string | AllowDownloads | verified: is_third_party_downloads_eligible echoed true |
| `thumbnail_url` | shared | upload/map | Extra |  |
| `timezone_offset` | shared | string | (set) |  |
| `title` | shared | upload/map | Extra |  |
| `tools` | shared | upload/map | Extra |  |
| `transcription_data` | shared | string | Extra |  |
| `trial_params` | reel | json | Trial | raw JSON {"graduation_strategy": "manual"|"ss_performance"|"time_limit"}; a string gets HTTP 500 "'str' object has no attribute 'get'"; HTTP 500 on a personal account, and the Reel may still be published as a normal public Reel |
| `upload_engine_config_enum` | shared | upload/map | Extra |  |
| `upload_id` | shared | string/upload/map | (set) |  |
| `video_edit_params` | shared | upload/map | Extra |  |
| `video_prompt_piece_id` | shared | upload/map | Extra |  |
| `video_result` | shared | string | Extra |  |
| `video_subtitles_enabled` | reel/shared | string | Captions | sent "1"; echo stayed false for a video without speech |
| `video_subtitles_translations_enabled` | reel | string | TranslateCaption | "1"/"0" |
| `visual_replies_params` | reel | json | Extra |  |
| `wearables_attribution_detection_method` | shared | string | Extra |  |
| `width` | shared | upload/map | Extra |  |
| `xmp_data` | shared | upload/map | Extra |  |
| `xsharing_user_ids` | shared | upload/map | Extra |  |
