package kube

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Rule struct {
	matchExp *regexp.Regexp // Compiled regex for matching file paths.
	pfl      *Profile       // Profile associated with the rule.

	Match   string `validate:"required" yaml:"match"`   // Regex to match file paths.
	Profile string `validate:"required" yaml:"profile"` // Profile name.
}

func NewRule(name, match, profile string) (*Rule, error) {
	r := &Rule{
		Match:   match,
		Profile: profile,
	}
	if err := r.CompileMatch(); err != nil {
		return nil, fmt.Errorf("rule %q: %w", name, err)
	}

	return r, nil
}

func MustNewRule(name, match, profile string) *Rule {
	r, err := NewRule(name, match, profile)
	if err != nil {
		panic(err)
	}

	return r
}

func (r *Rule) CompileMatch() error {
	if r.matchExp == nil {
		re, err := regexp.Compile(r.Match)
		if err != nil {
			return fmt.Errorf("compile match regex: %w", err)
		}
		r.matchExp = re
	}

	return nil
}

func (r *Rule) MatchPath(path string) bool {
	if r.matchExp == nil {
		panic(errors.New("rule missing a match expression"))
	}

	return r.matchExp.MatchString(path)
}

func (r *Rule) GetProfile() *Profile {
	if r.pfl == nil {
		panic(errors.New("rule missing a profile"))
	}

	return r.pfl
}

func (r *Rule) SetProfile(p *Profile) {
	r.pfl = p
}

func (r *Rule) String() string {
	profile := r.GetProfile()

	return fmt.Sprintf("%s: %s %s", r.Profile, profile.Command, strings.Join(profile.Args, " "))
}
