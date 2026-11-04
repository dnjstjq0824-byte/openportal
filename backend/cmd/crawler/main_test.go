package main

import "testing"

func TestApprovedTargetCannotGrantItselfCrawlRights(t *testing.T) {
	hosts := []string{"example.org", "xn--r8jz45g.xn--zckzah"}
	for _, target := range []string{"https://other.org/", "https://sub.example.org/", "http://example.org/", "https://example.org:8443/", "https://user:secret@example.org/", "invalid"} {
		if approvedTarget(target, hosts) == nil {
			t.Fatalf("accepted unapproved target %q", target)
		}
	}
	for _, target := range []string{"https://example.org/path?q=1", "https://EXAMPLE.ORG:443/", "https://例え.テスト/"} {
		if err := approvedTarget(target, hosts); err != nil {
			t.Fatalf("rejected approved target: %v", err)
		}
	}
}

func TestApprovedHostsDenyUnconfiguredAndInvalidTargets(t *testing.T) {
	for _, input := range []string{"", "localhost", "127.0.0.1", "https://example.org", "example.org:443", "example.org/private", "user@example.org", "example..org", "example.org,", "-bad.example.org"} {
		if hosts, err := approvedHosts(input); err == nil {
			t.Fatalf("accepted invalid allowlist %q: %v", input, hosts)
		}
	}
}

func TestApprovedHostsNormalizesAndDeduplicatesExplicitApproval(t *testing.T) {
	hosts, err := approvedHosts(" EXAMPLE.ORG, www.example.org,example.org ")
	if err != nil || len(hosts) != 2 || hosts[0] != "example.org" || hosts[1] != "www.example.org" {
		t.Fatalf("unexpected approved hosts: %v, %v", hosts, err)
	}
}

func TestApprovedHostsSupportsInternationalDomainNames(t *testing.T) {
	hosts, err := approvedHosts("例え.テスト")
	if err != nil || len(hosts) != 1 || hosts[0] != "xn--r8jz45g.xn--zckzah" {
		t.Fatalf("international hostname not normalized: %v, %v", hosts, err)
	}
}
