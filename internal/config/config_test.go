package config

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults with dsn from flag",
			args: []string{"-d", "postgres://localhost/db"},
			env:  map[string]string{},
			want: Config{
				RunAddress:     defaultRunAddress,
				DatabaseURI:    "postgres://localhost/db",
				AccrualAddress: "",
			},
		},
		{
			name: "flags override defaults",
			args: []string{"-a", "localhost:9090", "-d", "postgres://localhost/db", "-r", "localhost:8081"},
			env:  map[string]string{},
			want: Config{
				RunAddress:     "localhost:9090",
				DatabaseURI:    "postgres://localhost/db",
				AccrualAddress: "http://localhost:8081",
			},
		},
		{
			name: "env overrides flags",
			args: []string{"-a", "localhost:9090", "-d", "postgres://flag/db", "-r", "localhost:8081"},
			env: map[string]string{
				"RUN_ADDRESS":            "localhost:7070",
				"DATABASE_URI":           "postgres://env/db",
				"ACCRUAL_SYSTEM_ADDRESS": "localhost:9091",
			},
			want: Config{
				RunAddress:     "localhost:7070",
				DatabaseURI:    "postgres://env/db",
				AccrualAddress: "http://localhost:9091",
			},
		},
		{
			name: "accrual address already has scheme and trailing slash",
			args: []string{"-d", "postgres://localhost/db", "-r", "https://accrual.example.com/"},
			env:  map[string]string{},
			want: Config{
				RunAddress:     defaultRunAddress,
				DatabaseURI:    "postgres://localhost/db",
				AccrualAddress: "https://accrual.example.com",
			},
		},
		{
			name:    "empty database uri is an error",
			args:    []string{},
			env:     map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getenv := func(key string) string { return tt.env[key] }

			got, err := Parse(tt.args, getenv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() unexpected error: %v", err)
			}
			if *got != tt.want {
				t.Errorf("Parse() = %+v, want %+v", *got, tt.want)
			}
		})
	}
}
