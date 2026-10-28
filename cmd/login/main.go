// Command login performs one Instagram password login and saves the session.
// The password comes from the IG_PASSWORD environment variable, never a flag,
// so it stays out of shell history and process listings.
package main

import (
	"flag"
	"fmt"
	"os"

	goinsta "go-insta"
)

func main() {
	session := flag.String("session", "login.session.json", "session file (device fields + password key)")
	user := flag.String("user", "", "username or phone")
	// Attestation constants from the capture; override if you re-register keys.
	zcaKey := flag.String("zca-key", "keys/zca_key.pem", "zca signing key (PKCS8 PEM)")
	usdidKey := flag.String("usdid-key", "keys/usdid_key.pem", "usdid signing key (PKCS8 PEM)")
	lastUpload := flag.Int64("last-upload-ms", 1793119682768, "zca lastUploadedKeyTimeMs")
	usdidUUID := flag.String("usdid-uuid", "521604d9-eb45-4763-81a7-a748c024e43e", "usdid device uuid")
	flag.Parse()

	if *user == "" {
		fmt.Fprintln(os.Stderr, "need -user")
		os.Exit(2)
	}
	password := os.Getenv("IG_PASSWORD")
	if password == "" {
		fmt.Fprintln(os.Stderr, "set IG_PASSWORD")
		os.Exit(2)
	}

	insta, err := goinsta.ImportSession(*session)
	if err != nil {
		fmt.Fprintln(os.Stderr, "session:", err)
		os.Exit(1)
	}
	att, err := goinsta.NewAttestationFromFiles(*zcaKey, *usdidKey, "", *lastUpload, *usdidUUID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "attestation:", err)
		os.Exit(1)
	}
	if err := insta.SetAttestation(att); err != nil {
		fmt.Fprintln(os.Stderr, "set attestation:", err)
		os.Exit(1)
	}

	if err := insta.Login(*user, password); err != nil {
		fmt.Fprintln(os.Stderr, "login failed:", err)
		os.Exit(1)
	}
	if err := insta.ExportSession(*session); err != nil {
		fmt.Fprintln(os.Stderr, "save session:", err)
		os.Exit(1)
	}
	fmt.Printf("login ok; session saved to %s (user id %d)\n", *session, insta.Account.ID)
}
