package compliance

import "testing"

func TestValidateFQDN(t *testing.T) {
	valid := []string{
		"example.com",
		"sub.example.com",
		"deep.sub.example.com",
		"test-domain.co.uk",
		"a.bc",
		"x1.example.org",
	}
	for _, d := range valid {
		if !ValidateFQDN(d) {
			t.Errorf("expected %q to be valid FQDN", d)
		}
	}

	invalid := []string{
		"",
		"example",
		".example.com",
		"example.",
		"-example.com",
		"example-.com",
		"192.168.1.1",
		"*.example.com",
		"exam ple.com",
		"example..com",
		"a.b",
	}
	for _, d := range invalid {
		if ValidateFQDN(d) {
			t.Errorf("expected %q to be invalid FQDN", d)
		}
	}
}

func TestValidateDomains(t *testing.T) {
	valid, invalid := ValidateDomains([]string{
		"example.com",
		"  bad  ",
		"test.org",
		"",
		"  ",
	})
	if len(valid) != 2 {
		t.Fatalf("expected 2 valid, got %d", len(valid))
	}
	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d", len(invalid))
	}
	if invalid[0] != "bad" {
		t.Fatalf("expected invalid domain 'bad', got %q", invalid[0])
	}
}
