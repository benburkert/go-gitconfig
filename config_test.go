package gitconfig

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	want := []*Section{
		{
			Type: "user",
			Values: map[string]string{
				"name":       "Ben Burkert",
				"email":      "ben@benburkert.com",
				"signingkey": "BC8EDD7F",
			},
		},
		{
			Type: "color",
			Values: map[string]string{
				"ui": "auto",
			},
		},
		{
			Type: "color",
			ID:   "branch",
			Values: map[string]string{
				"current": "yellow reverse",
				"local":   "yellow",
				"remote":  "green",
			},
		},
		{
			Type: "color",
			ID:   "diff",
			Values: map[string]string{
				"meta":       "yellow bold",
				"frag":       "magenta bold",
				"old":        "red bold",
				"new":        "green bold",
				"whitespace": "red reverse",
			},
		},
		{
			Type: "color",
			ID:   "status",
			Values: map[string]string{
				"added":     "yellow",
				"changed":   "green",
				"untracked": "cyan",
			},
		},
		{
			Type: "core",
			Values: map[string]string{
				"whitespace": "fix,-indent-with-non-tab,trailing-space,cr-at-eol",
				"editor":     "/usr/bin/vim",
			},
		},
		{
			Type: "alias",
			Values: map[string]string{
				"ap":   "add -p",
				"s":    "status",
				"st":   "status",
				"c":    "commit -S",
				"br":   "branch",
				"co":   "checkout",
				"d":    "diff",
				"df":   "diff",
				"dc":   "diff --cached",
				"l":    "log --oneline",
				"lg":   "log -p",
				"lol":  "log --graph --decorate --pretty=oneline --abbrev-commit",
				"lola": "log --graph --decorate --pretty=oneline --abbrev-commit --all",
				"ls":   "ls-files",
				"ign":  "ls-files -o -i --exclude-standard",
			},
		},
		{
			Type: "include",
			Values: map[string]string{
				"path": ".github/.gitconfig",
			},
		},
		{
			Type: "credential",
			Values: map[string]string{
				"helper": "osxkeychain",
			},
		},
		{
			Type: "diff",
			Values: map[string]string{
				"tool": "vimdiff",
			},
		},
		{
			Type: "merge",
			Values: map[string]string{
				"tool": "vimdiff",
			},
		},
		{
			Type: "http",
			Values: map[string]string{
				"cookiefile": "/Users/benburkert/.gitcookies",
			},
		},
	}

	got, err := Parse(configData)
	if err != nil {
		t.Fatal(err)
	}

	if len(want) != len(got) {
		t.Errorf("want %d sections, got %d", len(want), len(got))
	}

	for i, _ := range got {
		if !reflect.DeepEqual(want[i], got[i]) {
			t.Errorf("want section %#v, got %#v", want[i], got[i])
		}
	}
}

var (
	configData = []byte(`[user]
  name = Ben Burkert
  email = ben@benburkert.com
  signingkey = BC8EDD7F
[color]
    ui = auto #true
  [color "branch"]
    current = yellow reverse
    local = yellow
    remote = green
  [color "diff"]
    meta = yellow bold
    frag = magenta bold
    old = red bold
    new = green bold
    whitespace = red reverse
  [color "status"]
    added = yellow
    changed = green
    untracked = cyan
[core]
  whitespace=fix,-indent-with-non-tab,trailing-space,cr-at-eol
  editor = /usr/bin/vim
[alias]
  ap = add -p
  s = status
  st = status
  c = commit -S
  br = branch
  co = checkout
  d = diff
  df = diff
  dc = diff --cached
  l = log --oneline
  lg = log -p
  lol = log --graph --decorate --pretty=oneline --abbrev-commit
  lola = log --graph --decorate --pretty=oneline --abbrev-commit --all
  ls = ls-files
  ign = ls-files -o -i --exclude-standard
[include]
  path = .github/.gitconfig
[credential]
	helper = osxkeychain
[diff]
	tool = vimdiff
[merge]
	tool = vimdiff
[http]
	cookiefile = /Users/benburkert/.gitcookies
`)
)
