package luhn

import "testing"

func TestValid(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{name: "valid 12345678903", number: "12345678903", want: true},
		{name: "valid 9278923470", number: "9278923470", want: true},
		{name: "too short and invalid", number: "123", want: false},
		{name: "non-digit characters", number: "abc", want: false},
		{name: "empty string", number: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Valid(tt.number); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.number, got, tt.want)
			}
		})
	}
}

func BenchmarkValid(b *testing.B) {
	for b.Loop() {
		Valid("12345678903")
	}
}
