# cmdline-mastodon

A tiny command-line Mastodon client written in Go. Log in to any Mastodon server, read your timeline, and publish, reply to, or delete posts from the terminal.

Zero external dependencies — just the Go standard library.

## Build

Requires Go 1.22+.

```sh
go build -o masto .
```

Optionally put the binary on your `$PATH`:

```sh
mv masto /usr/local/bin/
```

## Login

```sh
masto login mastodon.social
```

This registers an app on the server, prints an authorization URL, and waits for you to paste back the code shown after authorizing in your browser. The resulting access token is written to `~/.config/cmdline-mastodon/config.json` (mode `0600`).

Any Mastodon-API-compatible server works (e.g. `mastodon.social`, `hachyderm.io`, your own instance). You can pass a bare hostname or a full URL.

## Commands

```
masto login <server>
masto post <text> [--visibility public|unlisted|private|direct] [--cw <text>] [--reply-to <id>]
masto timeline [--kind home|public] [--limit N]
masto reply <status-id> <text>
masto delete <status-id>
masto whoami
```

### Examples

Post a status:

```sh
masto post "Hello from the terminal"
```

Post with a content warning and unlisted visibility:

```sh
masto post "spoilers for episode 3" --cw "TV spoilers" --visibility unlisted
```

Read the 10 most recent posts from your home timeline:

```sh
masto timeline --limit 10
```

Read the public (federated) timeline:

```sh
masto timeline --kind public
```

Reply to a status (use the ID shown by `timeline`):

```sh
masto reply 110000000000000001 "agreed!"
```

Delete one of your own statuses:

```sh
masto delete 110000000000000001
```

Check who you're logged in as:

```sh
masto whoami
```

## Configuration

State lives in a single JSON file:

```
~/.config/cmdline-mastodon/config.json
```

It contains the server URL, the app's client ID/secret, and the user access token. To log out, delete the file.

## Scopes

The app registers with `read write` scopes. It does not request `follow` or `admin`.

## Project layout

```
main.go        - command dispatch
commands.go    - subcommand handlers + timeline HTML rendering
client.go      - Mastodon API client
config.go      - config load/save
```
