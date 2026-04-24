package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
)

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// parseIntersperse parses flags that may appear before, after, or between
// positional arguments. Go's flag package stops at the first non-flag token,
// so we loop: parse, peel off one positional, parse the rest.
func parseIntersperse(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
	return positional, nil
}

func readStatusText(arg string) (string, error) {
	if arg != "-" {
		return arg, nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	text := strings.TrimRight(string(b), "\n")
	if text == "" {
		return "", fmt.Errorf("empty status from stdin")
	}
	return text, nil
}

func normalizeServer(s string) string {
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimRight(s, "/")
}

func cmdLogin(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: masto login <server>")
	}
	server := normalizeServer(args[0])
	redirect := "urn:ietf:wg:oauth:2.0:oob"
	scopes := "read write"

	c := newClient(server, "")
	app, err := c.RegisterApp("cmdline-mastodon", scopes, redirect)
	if err != nil {
		return fmt.Errorf("register app: %w", err)
	}

	q := url.Values{}
	q.Set("client_id", app.ClientID)
	q.Set("scope", scopes)
	q.Set("redirect_uri", redirect)
	q.Set("response_type", "code")
	authURL := server + "/oauth/authorize?" + q.Encode()

	fmt.Println("Open this URL in your browser, authorize the app, and paste the code below:")
	fmt.Println()
	fmt.Println("  " + authURL)
	fmt.Println()
	fmt.Print("Code: ")

	r := bufio.NewReader(os.Stdin)
	code, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("empty code")
	}

	token, err := c.ExchangeCode(app.ClientID, app.ClientSecret, code, redirect, scopes)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	cfg := &Config{
		Server:       server,
		ClientID:     app.ClientID,
		ClientSecret: app.ClientSecret,
		AccessToken:  token,
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}

	who, err := newClient(server, token).VerifyCredentials()
	if err != nil {
		fmt.Println("Logged in.")
		return nil
	}
	fmt.Printf("Logged in as @%s on %s\n", who.Username, server)
	return nil
}

func cmdPost(args []string) error {
	fs := flag.NewFlagSet("post", flag.ContinueOnError)
	visibility := fs.String("visibility", "", "public|unlisted|private|direct")
	cw := fs.String("cw", "", "content warning / spoiler text")
	replyTo := fs.String("reply-to", "", "status ID to reply to")
	var media stringList
	fs.Var(&media, "media", "path to image to attach (repeat for up to 4)")
	rest, err := parseIntersperse(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: masto post <text> [flags]  (use - to read from stdin)")
	}
	if len(media) > 4 {
		return fmt.Errorf("at most 4 images allowed, got %d", len(media))
	}
	text, err := readStatusText(rest[0])
	if err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	c := newClient(cfg.Server, cfg.AccessToken)
	var mediaIDs []string
	for _, p := range media {
		fmt.Fprintf(os.Stderr, "uploading %s...\n", p)
		id, err := c.UploadMedia(p)
		if err != nil {
			return err
		}
		mediaIDs = append(mediaIDs, id)
	}
	s, err := c.Post(PostParams{
		Status:      text,
		Visibility:  *visibility,
		SpoilerText: *cw,
		InReplyToID: *replyTo,
		MediaIDs:    mediaIDs,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Posted: %s\n", s.URL)
	fmt.Printf("ID: %s\n", s.ID)
	return nil
}

func cmdTimeline(args []string) error {
	fs := flag.NewFlagSet("timeline", flag.ContinueOnError)
	kind := fs.String("kind", "home", "home|public")
	limit := fs.Int("limit", 20, "number of statuses to fetch")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ss, err := newClient(cfg.Server, cfg.AccessToken).Timeline(*kind, *limit)
	if err != nil {
		return err
	}
	for _, s := range ss {
		name := s.Account.DisplayName
		if name == "" {
			name = s.Account.Username
		}
		fmt.Printf("─── %s  %s (@%s)  %s\n", s.ID, name, s.Account.Acct, s.CreatedAt)
		if s.SpoilerText != "" {
			fmt.Printf("CW: %s\n", s.SpoilerText)
		}
		fmt.Println(stripHTML(s.Content))
		fmt.Println()
	}
	return nil
}

func cmdPosts(args []string) error {
	fs := flag.NewFlagSet("posts", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "number of statuses to fetch")
	excludeReplies := fs.Bool("exclude-replies", false, "skip replies")
	excludeReblogs := fs.Bool("exclude-reblogs", false, "skip boosts")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	c := newClient(cfg.Server, cfg.AccessToken)
	who, err := c.VerifyCredentials()
	if err != nil {
		return err
	}
	ss, err := c.AccountStatuses(who.ID, *limit, *excludeReplies, *excludeReblogs)
	if err != nil {
		return err
	}
	for _, s := range ss {
		fmt.Printf("─── %s  %s  [%s]\n", s.ID, s.CreatedAt, s.Visibility)
		if s.SpoilerText != "" {
			fmt.Printf("CW: %s\n", s.SpoilerText)
		}
		fmt.Println(stripHTML(s.Content))
		if s.URL != "" {
			fmt.Println(s.URL)
		}
		fmt.Println()
	}
	return nil
}

func cmdReply(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: masto reply <status-id> <text>  (use - to read from stdin)")
	}
	text, err := readStatusText(args[1])
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	s, err := newClient(cfg.Server, cfg.AccessToken).Post(PostParams{
		Status:      text,
		InReplyToID: args[0],
	})
	if err != nil {
		return err
	}
	fmt.Printf("Replied: %s\n", s.URL)
	fmt.Printf("ID: %s\n", s.ID)
	return nil
}

func cmdDelete(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: masto delete <status-id>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := newClient(cfg.Server, cfg.AccessToken).Delete(args[0]); err != nil {
		return err
	}
	fmt.Println("Deleted.")
	return nil
}

func cmdWhoami(args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	who, err := newClient(cfg.Server, cfg.AccessToken).VerifyCredentials()
	if err != nil {
		return err
	}
	fmt.Printf("@%s (%s) on %s\n", who.Username, who.DisplayName, cfg.Server)
	return nil
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "</p><p>", "\n\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(s)
}
