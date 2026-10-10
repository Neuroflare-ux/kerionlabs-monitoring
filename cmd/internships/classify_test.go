package main

import "testing"

func TestIsEntryLevel(t *testing.T) {
	cases := map[string]bool{
		"Software Engineer Intern":     true,
		"Junior DevOps Engineer":       true,
		"Graduate Cloud Engineer":      true,
		"Security Analyst I":           true,
		"Senior Software Engineer":     false,
		"Internal Tools Engineer":      false,
		"Associate Director, Security": false,
		"Staff SRE":                    false,
		"Product Manager Intern":       false,
	}
	for title, want := range cases {
		if got := isEntryLevel(title); got != want {
			t.Errorf("isEntryLevel(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestCategorize(t *testing.T) {
	cases := map[string]Category{
		"Cloud Engineer Intern":    CatCloudDevOps,
		"DevOps Intern":            CatCloudDevOps,
		"Security Analyst Intern":  CatSecurity,
		"IT Support Intern":        CatITSupport,
		"Software Engineer Intern": CatSoftware,
		"Marketing Intern":         "",
	}
	for title, want := range cases {
		if got := categorize(title); got != want {
			t.Errorf("categorize(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestEligibility(t *testing.T) {
	cases := []struct {
		loc  string
		want Eligibility
		ok   bool
	}{
		{"Nairobi, Kenya", EligKenya, true},
		{"Nairobi, Kenya; London, UK", EligKenya, true},
		{"Remote - Africa", EligRemoteGlobal, true},
		{"Anywhere", EligRemoteGlobal, true},
		{"Remote", EligRemoteCheck, true},
		{"Remote - EMEA", EligRemoteCheck, true},
		{"Remote - US", "", false},
		{"Remote - Colombia", "", false},
		{"Remote - Brazil", "", false},
		{"Remote (Remote) ", EligRemoteCheck, true},
		{"Remote, Canada", "", false},
		{"San Francisco, CA", "", false},
		{"London, UK", "", false},
	}
	for _, c := range cases {
		got, ok := eligibility(c.loc)
		if got != c.want || ok != c.ok {
			t.Errorf("eligibility(%q) = (%q,%v), want (%q,%v)", c.loc, got, ok, c.want, c.ok)
		}
	}
}