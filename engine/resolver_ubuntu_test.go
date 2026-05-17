package engine

import "testing"

func TestParseUbuntuSHA256SUMS(t *testing.T) {
	const fakeSums = "" +
		"abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1  *jammy-server-cloudimg-amd64.img\n" +
		"def456def456def456def456def456def456def456def456def456def456def4  *jammy-server-cloudimg-arm64.img\n" +
		"ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi7  jammy-server-cloudimg-amd64-disk-kvm.img\n"

	tests := []struct {
		name     string
		body     string
		filename string
		want     string
	}{
		{
			name:     "amd64 with asterisk prefix",
			body:     fakeSums,
			filename: "jammy-server-cloudimg-amd64.img",
			want:     "abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abc1",
		},
		{
			name:     "arm64 with asterisk prefix",
			body:     fakeSums,
			filename: "jammy-server-cloudimg-arm64.img",
			want:     "def456def456def456def456def456def456def456def456def456def456def4",
		},
		{
			name:     "filename without asterisk prefix",
			body:     fakeSums,
			filename: "jammy-server-cloudimg-amd64-disk-kvm.img",
			want:     "ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi789ghi7",
		},
		{
			name:     "not found returns empty string",
			body:     fakeSums,
			filename: "jammy-server-cloudimg-riscv64.img",
			want:     "",
		},
		{
			name:     "empty body",
			body:     "",
			filename: "jammy-server-cloudimg-amd64.img",
			want:     "",
		},
		{
			name:     "malformed lines skipped",
			body:     "onlyonecolumn\n",
			filename: "jammy-server-cloudimg-amd64.img",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUbuntuSHA256SUMS(tt.body, tt.filename)
			if got != tt.want {
				t.Errorf("parseUbuntuSHA256SUMS(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
