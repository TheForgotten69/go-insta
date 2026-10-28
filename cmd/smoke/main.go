// Command smoke runs the API methods against a logged-in session and reports
// which ones work. By default it only reads. -writes adds reversible pairs on
// your own account: follow then unfollow, and upload a photo, comment on it,
// delete the comment and delete the photo.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"time"

	goinsta "go-insta"
)

func main() {
	session := flag.String("session", "login.session.json", "logged-in session file")
	zcaKey := flag.String("zca-key", "keys/zca_key.pem", "zca signing key (PKCS8 PEM)")
	usdidKey := flag.String("usdid-key", "keys/usdid_key.pem", "usdid signing key (PKCS8 PEM)")
	lastUpload := flag.Int64("last-upload-ms", 1793119682768, "zca lastUploadedKeyTimeMs")
	usdidUUID := flag.String("usdid-uuid", "521604d9-eb45-4763-81a7-a748c024e43e", "usdid device uuid")
	target := flag.String("user", "instagram", "public account to look up")
	pause := flag.Duration("pause", 2*time.Second, "delay between calls")
	writes := flag.Bool("writes", false, "also run reversible write pairs (follow/unfollow, post/comment/delete)")
	flag.Parse()

	insta, err := goinsta.ImportSession(*session)
	if err != nil {
		fail("session", err)
	}
	att, err := goinsta.NewAttestationFromFiles(*zcaKey, *usdidKey, "", *lastUpload, *usdidUUID)
	if err != nil {
		fail("attestation", err)
	}
	if err := insta.SetAttestation(att); err != nil {
		fail("set attestation", err)
	}

	var ok, bad int
	step := func(name string, fn func() (string, error)) {
		summary, err := fn()
		if err != nil {
			bad++
			fmt.Printf("FAIL  %-22s %v\n", name, err)
		} else {
			ok++
			fmt.Printf("ok    %-22s %s\n", name, summary)
		}
		time.Sleep(*pause)
	}

	acc := insta.Account
	step("account.Sync", func() (string, error) {
		if err := acc.Sync(); err != nil {
			return "", err
		}
		return fmt.Sprintf("@%s (%s), %d followers", acc.Username, acc.FullName, acc.FollowerCount), nil
	})
	step("account.ReelSettings", func() (string, error) {
		s, err := acc.ReelSettings()
		if err != nil {
			return "", err
		}
		return "auto_archive=" + s.ReelAutoArchive, nil
	})
	step("account.Stories", func() (string, error) {
		s, err := acc.Stories()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d own story items", len(s.Reel.Items)), nil
	})
	step("GetAccountFamily", func() (string, error) {
		b, err := insta.GetAccountFamily()
		return fmt.Sprintf("%d bytes", len(b)), err
	})

	var firstPost *goinsta.Item
	step("timeline.Next", func() (string, error) {
		tl := insta.Timeline
		if !tl.Next() {
			if err := tl.Error(); err != nil && !errors.Is(err, goinsta.ErrNoMore) {
				return "", err
			}
		}
		if len(tl.Items) > 0 {
			firstPost = tl.Items[0]
		}
		return fmt.Sprintf("%d items, more=%v", len(tl.Items), tl.MoreAvailable), nil
	})
	step("timeline.FetchTray", func() (string, error) {
		if err := insta.Timeline.FetchTray("cold_start"); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d story reels in tray", len(insta.Timeline.Stories())), nil
	})
	if firstPost != nil {
		step("item.Comments", func() (string, error) {
			c, err := firstPost.Comments()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d comments on %s", len(c.Items), firstPost.ID), nil
		})
	}

	var user *goinsta.User
	step("SearchUsers", func() (string, error) {
		r, err := insta.SearchUsers(*target)
		if err != nil {
			return "", err
		}
		for _, u := range r.Users {
			if u.Username == *target {
				user = u
			}
		}
		if user == nil && len(r.Users) > 0 {
			user = r.Users[0]
		}
		return fmt.Sprintf("%d results", len(r.Users)), nil
	})
	if user != nil {
		step("user.Info", func() (string, error) {
			if err := user.Info(); err != nil {
				return "", err
			}
			return fmt.Sprintf("@%s id=%d", user.Username, user.ID), nil
		})
		step("user.GetFriendship", func() (string, error) {
			f, err := user.GetFriendship()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("following=%v", f.Following), nil
		})
		step("Friendships", func() (string, error) {
			m, err := insta.Friendships(int64(user.ID))
			return fmt.Sprintf("%d entries", len(m)), err
		})
		step("user.Posts", func() (string, error) {
			p, err := user.Posts("")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d posts, more=%v", len(p.Items), p.MoreAvailable), nil
		})
		step("user.Followers", func() (string, error) {
			l, err := user.Followers("")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d users", len(l.Users)), nil
		})
		step("user.Following", func() (string, error) {
			l, err := user.Following("")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d users", len(l.Users)), nil
		})
		step("user.Stories", func() (string, error) {
			s, err := user.Stories()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d items", len(s.Reel.Items)), nil
		})
		step("user.Highlights", func() (string, error) {
			h, err := user.Highlights()
			return fmt.Sprintf("%d highlights", len(h)), err
		})
		step("user.SuggestedUsers", func() (string, error) {
			s, err := user.SuggestedUsers()
			return fmt.Sprintf("%d users", len(s)), err
		})
	}

	step("collections.Next", func() (string, error) {
		c := insta.Collections
		c.Next()
		if err := c.Error(); err != nil && !errors.Is(err, goinsta.ErrNoMore) {
			return "", err
		}
		return fmt.Sprintf("%d collections", len(c.Items)), nil
	})
	step("activity.Next", func() (string, error) {
		a := insta.Activity
		a.Next()
		if err := a.Error(); err != nil && !errors.Is(err, goinsta.ErrNoMore) {
			return "", err
		}
		return "inbox fetched", nil
	})
	step("reels.Next", func() (string, error) {
		r := insta.Reels()
		if err := r.Next(); err != nil {
			return "", err
		}
		return fmt.Sprintf("%d reels", len(r.Items)), nil
	})

	if *writes {
		if user != nil {
			step("user.Follow", func() (string, error) { return "@" + user.Username, user.Follow() })
			step("user.Unfollow", func() (string, error) { return "@" + user.Username, user.Unfollow() })
		}
		var post *goinsta.Item
		step("UploadPhoto", func() (string, error) {
			img, err := testJPEG()
			if err != nil {
				return "", err
			}
			post, err = insta.UploadPhoto(bytes.NewReader(img), "go-insta smoke test")
			if err != nil {
				return "", err
			}
			return post.ID, nil
		})
		if post != nil {
			var comment *goinsta.Comment
			step("item.Comment", func() (string, error) {
				var err error
				comment, err = post.Comment("smoke test")
				if err != nil {
					return "", err
				}
				return fmt.Sprint(comment.ID), nil
			})
			if comment != nil {
				step("comment.Delete", func() (string, error) { return "", comment.Delete() })
			}
			step("item.Delete", func() (string, error) { return post.ID, post.Delete() })
		}
	}

	if err := insta.ExportSession(*session); err != nil {
		fmt.Fprintln(os.Stderr, "save session:", err)
	}
	fmt.Printf("\n%d ok, %d failed\n", ok, bad)
	if bad > 0 {
		os.Exit(1)
	}
}

// testJPEG draws a 1080x1080 gradient, a size the feed accepts without cropping.
func testJPEG() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1080, 1080))
	for y := 0; y < 1080; y++ {
		for x := 0; x < 1080; x++ {
			img.Set(x, y, color.RGBA{uint8(x / 5), uint8(y / 5), 160, 255})
		}
	}
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	return buf.Bytes(), err
}

func fail(what string, err error) {
	fmt.Fprintln(os.Stderr, what+":", err)
	os.Exit(1)
}
