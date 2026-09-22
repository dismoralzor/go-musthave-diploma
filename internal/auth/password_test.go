package auth

import "testing"

func TestHashAndCheck(t *testing.T) {
	hash, err := Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash() unexpected error: %v", err)
	}

	if !Check(hash, "correct-password") {
		t.Error("Check() = false for correct password, want true")
	}

	if Check(hash, "wrong-password") {
		t.Error("Check() = true for wrong password, want false")
	}
}
