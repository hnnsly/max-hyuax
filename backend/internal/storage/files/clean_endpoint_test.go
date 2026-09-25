package files

import "testing"

func TestCleanEndpoint(t *testing.T) {
	tests := []struct {
		in     string
		useSSL bool
		want   string
	}{
		{in: "s3.example.com", useSSL: true, want: "s3.example.com"},
		{in: "s3.example.com:443", useSSL: true, want: "s3.example.com"},
		{in: "https://s3.example.com:443/", useSSL: true, want: "s3.example.com"},
		{in: "http://s3.example.com:80/", useSSL: false, want: "s3.example.com"},
		{in: "minio:9000", useSSL: false, want: "minio:9000"},
		{in: "localhost:19000", useSSL: false, want: "localhost:19000"},
	}
	for _, tc := range tests {
		got := cleanEndpoint(tc.in, tc.useSSL)
		if got != tc.want {
			t.Errorf("cleanEndpoint(%q, %v) = %q, want %q", tc.in, tc.useSSL, got, tc.want)
		}
	}
}
