// Command httplog inspects a device HTTP capture (log_http.txt).
//
//	httplog list    log_http.txt                       endpoint inventory
//	httplog dump    [-raw] [-match s] [-instagram] log_http.txt   entries as JSON (scrubbed unless -raw)
//	httplog session log_http.txt > session.json        session for goinsta.ImportSession
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/TheForgotten69/go-insta/internal/httplog"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	raw := fs.Bool("raw", false, "do not scrub credentials")
	match := fs.String("match", "", "only entries whose URL or friendly name matches this regexp")
	igOnly := fs.Bool("instagram", false, "only *.instagram.com hosts")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}

	f, err := os.Open(fs.Arg(0))
	check(err)
	entries, err := httplog.Parse(f)
	f.Close()
	check(err)

	sess, sessErr := httplog.ExtractSession(entries)

	switch cmd {
	case "list":
		list(entries)
	case "dump":
		var re *regexp.Regexp
		if *match != "" {
			re = regexp.MustCompile(*match)
		}
		out := []httplog.Entry{}
		for _, e := range entries {
			if *igOnly && !strings.HasSuffix(e.Host, "instagram.com") {
				continue
			}
			if re != nil && !re.MatchString(e.URL) && !re.MatchString(e.ReqHeaders["x-fb-friendly-name"]) {
				continue
			}
			if !*raw {
				e = httplog.Scrub(e, sess)
			}
			out = append(out, e)
		}
		writeJSON(out)
	case "session":
		check(sessErr)
		writeJSON(sess)
	default:
		usage()
	}
}

var idRe = regexp.MustCompile(`\d{6,}(_\d+)?`)

func list(entries []httplog.Entry) {
	type row struct {
		key      string
		n        int
		statuses map[int]int
	}
	rows := map[string]*row{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Host, "instagram.com") {
			continue
		}
		key := e.Method + " " + e.Host + idRe.ReplaceAllString(e.Path, "{id}")
		if fn := e.ReqHeaders["x-fb-friendly-name"]; strings.Contains(e.Path, "graphql") && fn != "" {
			key += "  [" + fn + "]"
		}
		r := rows[key]
		if r == nil {
			r = &row{key: key, statuses: map[int]int{}}
			rows[key] = r
		}
		r.n++
		r.statuses[e.Status]++
	}
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r := rows[k]
		fmt.Printf("%4d  %v  %s\n", r.n, r.statuses, k)
	}
}

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	check(enc.Encode(v))
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "httplog:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: httplog list|dump|session [-raw] [-match re] [-instagram] <log_http.txt>")
	os.Exit(2)
}
