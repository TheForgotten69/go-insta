// Command login performs one Instagram password login and saves the session.
// The password comes from the IG_PASSWORD environment variable, never a flag,
// so it stays out of shell history and process listings. If the account has
// two-factor on, it prompts for a code, or reads IG_TOTP_SECRET (the base32 key
// from the authenticator setup) to generate one non-interactively.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

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
	err = insta.Login(ctx, *user, password)
	if errors.Is(err, goinsta.ErrTwoFactorRequired) {
		code, cerr := twoFactorCode()
		if cerr != nil {
			fmt.Fprintln(os.Stderr, "two-factor:", cerr)
			os.Exit(1)
		}
		err = insta.LoginTwoFactor(ctx, code)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "login failed:", err)
		os.Exit(1)
	}
	if err := insta.ExportSession(*session); err != nil {
		fmt.Fprintln(os.Stderr, "save session:", err)
		os.Exit(1)
	}
	fmt.Printf("login ok; session saved to %s (user id %d)\n", *session, insta.Account.ID)
}

// twoFactorCode generates a code from IG_TOTP_SECRET, or prompts for one.
func twoFactorCode() (string, error) {
	if secret := os.Getenv("IG_TOTP_SECRET"); secret != "" {
		return goinsta.TOTP(secret, time.Now())
	}
	fmt.Fprint(os.Stderr, "2FA code: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
