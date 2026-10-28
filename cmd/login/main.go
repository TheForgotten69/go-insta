// Command login performs one Instagram password login and saves the session.
// The password comes from the IG_PASSWORD environment variable, never a flag,
// so it stays out of shell history and process listings.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	goinsta "go-insta"
)

func main() {
	session := flag.String("session", "login.session.json", "session file (device fields, password key, attestation keys)")
	user := flag.String("user", "", "username or phone")
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	insta, err := goinsta.ImportSession(*session)
	if err != nil {
		fmt.Fprintln(os.Stderr, "session:", err)
		os.Exit(1)
	}
	if err := insta.Login(ctx, *user, password); err != nil {
		fmt.Fprintln(os.Stderr, "login failed:", err)
		os.Exit(1)
	}
	if err := insta.ExportSession(*session); err != nil {
		fmt.Fprintln(os.Stderr, "save session:", err)
		os.Exit(1)
	}
	fmt.Printf("login ok; session saved to %s (user id %d)\n", *session, insta.Account.ID)
}
