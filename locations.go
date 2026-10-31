package goinsta

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type locationSearchQuery struct {
	SearchQuery string `json:"search_query"`
	Latitude    string `json:"latitude"`
	Longitude   string `json:"longitude"`
	RankToken   string `json:"rankToken,omitempty"`
}

type venue struct {
	ExternalID     Int64   `json:"external_id"`
	ExternalSource string  `json:"external_id_source"`
	Name           string  `json:"name"`
	Address        string  `json:"address"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
}

// SearchLocations finds places by name around lat/lng; 0, 0 searches without a
// position, as the app does without location access.
func (insta *Instagram) SearchLocations(ctx context.Context, query string, lat, lng float64) ([]Location, error) {
	var resp struct {
		Venues []venue `json:"venues"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: urlLocationSearch, Form: locationSearchQuery{SearchQuery: query, Latitude: coordinate(lat, lng, lat), Longitude: coordinate(lat, lng, lng)}}, &resp); err != nil {
		return nil, err
	}
	out := make([]Location, len(resp.Venues))
	for i, v := range resp.Venues {
		out[i] = Location{
			ID: v.ExternalID, Name: v.Name, Address: v.Address, Lat: v.Lat, Lng: v.Lng,
			ExternalSource: v.ExternalSource, FacebookPlacesID: v.ExternalID,
		}
	}
	return out, nil
}

// coordinate formats v like Java's String.valueOf(double), or "0.000000" when
// there is no position at all.
func coordinate(lat, lng, v float64) string {
	if lat == 0 && lng == 0 {
		return "0.000000"
	}
	s := strconv.FormatFloat(v, 'f', -1, 64)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// LocationInfo loads a location by id.
func (insta *Instagram) LocationInfo(ctx context.Context, id int64) (*Location, error) {
	var resp struct {
		Location *Location `json:"location"`
	}
	if err := insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlLocationInfo, id)}, &resp); err != nil {
		return nil, err
	}
	if resp.Location == nil {
		return nil, fmt.Errorf("location %d: %w", id, ErrEmptyResponse)
	}
	return resp.Location, nil
}

// LocationTab is the order of a location's feed.
type LocationTab string

const (
	LocationTabRanked LocationTab = "ranked"
	LocationTabRecent LocationTab = "recent"
)

type locationSectionsRequest struct {
	UUID                string `json:"_uuid"`
	SessionID           string `json:"session_id"`
	Tab                 string `json:"tab"`
	ExcludeBloksWidgets string `json:"exclude_bloks_widgets"`
	MaxID               string `json:"max_id,omitempty"`
	Page                string `json:"page,omitempty"`
	NextMediaIDs        string `json:"next_media_ids,omitempty"`
}

type locationSection struct {
	LayoutContent struct {
		Medias []struct {
			Media *Item `json:"media"`
		} `json:"medias"`
		FillItems []struct {
			Media *Item `json:"media"`
		} `json:"fill_items"`
	} `json:"layout_content"`
}

// LocationFeed pages the posts at a location, ranked or most recent first.
func (insta *Instagram) LocationFeed(id int64, tab LocationTab) *Pager[*Item] {
	sessionID := generateUUID()
	var maxID, page, nextIDs string
	var pos int
	return newPager(func(ctx context.Context) ([]*Item, string, error) {
		var resp struct {
			Sections      []locationSection `json:"sections"`
			MoreAvailable bool              `json:"more_available"`
			NextMaxID     string            `json:"next_max_id"`
			NextPage      int               `json:"next_page"`
			NextMediaIDs  []Int64           `json:"next_media_ids"`
		}
		err := insta.callJSON(ctx, &reqOptions{Endpoint: fmt.Sprintf(urlLocationSections, id), IsPost: true, Form: locationSectionsRequest{
			UUID: insta.uuid, SessionID: sessionID, Tab: string(tab), ExcludeBloksWidgets: "true",
			MaxID: maxID, Page: page, NextMediaIDs: nextIDs,
		}}, &resp)
		if err != nil {
			return nil, "", err
		}
		var items []*Item
		for _, s := range resp.Sections {
			for _, m := range s.LayoutContent.Medias {
				items = append(items, m.Media)
			}
			for _, m := range s.LayoutContent.FillItems {
				items = append(items, m.Media)
			}
		}
		out := items[:0]
		for _, it := range items {
			if it == nil {
				continue
			}
			setToItem(it, insta, "location_feed")
			it.Index = pos
			pos++
			out = append(out, it)
		}
		maxID, page, nextIDs = resp.NextMaxID, strconv.Itoa(resp.NextPage), javaList(resp.NextMediaIDs)
		return out, cursorIf(resp.MoreAvailable, maxID), nil
	})
}

// javaList formats ids like Java's List.toString(): "[1, 2]".
func javaList(ids []Int64) string {
	if len(ids) == 0 {
		return ""
	}
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.FormatInt(int64(id), 10)
	}
	return "[" + strings.Join(s, ", ") + "]"
}
