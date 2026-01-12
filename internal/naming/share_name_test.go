package naming

import "testing"

func TestComputeShareNameDefault(t *testing.T) {
	got, err := ComputeShareName("team", "data", "")
	if err != nil {
		t.Fatalf("ComputeShareName error = %v", err)
	}
	if got != "team-data" {
		t.Fatalf("ComputeShareName = %q, want %q", got, "team-data")
	}
}

func TestComputeShareNameSanitizes(t *testing.T) {
	got, err := ComputeShareName("Team", "Data_01", "")
	if err != nil {
		t.Fatalf("ComputeShareName error = %v", err)
	}
	if got != "team-data-01" {
		t.Fatalf("ComputeShareName = %q, want %q", got, "team-data-01")
	}
}

func TestComputeShareNameOverride(t *testing.T) {
	got, err := ComputeShareName("team", "data", "Custom.Share")
	if err != nil {
		t.Fatalf("ComputeShareName error = %v", err)
	}
	if got != "custom-share" {
		t.Fatalf("ComputeShareName = %q, want %q", got, "custom-share")
	}
}

func TestComputeShareNameTooLong(t *testing.T) {
	long := "this-is-a-very-long-namespace-name-for-testing-share-naming"
	got, err := ComputeShareName(long, long, "")
	if err != nil {
		t.Fatalf("ComputeShareName error = %v", err)
	}
	if len(got) != maxShareNameLength {
		t.Fatalf("length = %d, want %d", len(got), maxShareNameLength)
	}

	sanitized, err := Sanitize(long + "-" + long)
	if err != nil {
		t.Fatalf("Sanitize error = %v", err)
	}
	expectedSuffix := "-" + hashString(sanitized)
	if got[len(got)-len(expectedSuffix):] != expectedSuffix {
		t.Fatalf("suffix = %q, want %q", got[len(got)-len(expectedSuffix):], expectedSuffix)
	}
}

func TestComputeShareNameInvalid(t *testing.T) {
	_, err := ComputeShareName("---", "***", "")
	if err == nil {
		t.Fatalf("ComputeShareName error = nil, want error")
	}
}
