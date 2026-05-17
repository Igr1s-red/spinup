package engine

import (
	"strings"
	"testing"
)

func TestParseDebianSHA512SUMS(t *testing.T) {
	const fakeSums = "" +
		"aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aa  debian-12-generic-amd64-20240101-1234.qcow2\n" +
		"bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bb  *debian-12-generic-arm64-20240101-1234.qcow2\n" +
		"ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333ccc333cc  debian-12-genericcloud-amd64-20240101-1234.qcow2\n"

	tests := []struct {
		name         string
		body         string
		debArch      string
		wantFilename string
		wantChecksum string
		wantErr      bool
	}{
		{
			name:         "finds amd64 without asterisk prefix",
			body:         fakeSums,
			debArch:      "amd64",
			wantFilename: "debian-12-generic-amd64-20240101-1234.qcow2",
			wantChecksum: "aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aa",
		},
		{
			name:         "finds arm64 with asterisk prefix",
			body:         fakeSums,
			debArch:      "arm64",
			wantFilename: "debian-12-generic-arm64-20240101-1234.qcow2",
			wantChecksum: "bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bbb222bb",
		},
		{
			name:    "arch not present",
			body:    fakeSums,
			debArch: "riscv64",
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    "",
			debArch: "amd64",
			wantErr: true,
		},
		{
			name:    "malformed lines ignored",
			body:    "onlyonecolumn\n",
			debArch: "amd64",
			wantErr: true,
		},
		{
			name:         "genericcloud not matched — generic matched first",
			body:         fakeSums,
			debArch:      "amd64",
			wantFilename: "debian-12-generic-amd64-20240101-1234.qcow2",
			wantChecksum: "aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aaa111aa",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename, checksum, err := parseDebianSHA512SUMS(strings.NewReader(tt.body), tt.debArch)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseDebianSHA512SUMS() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if filename != tt.wantFilename {
				t.Errorf("filename = %q, want %q", filename, tt.wantFilename)
			}
			if checksum != tt.wantChecksum {
				t.Errorf("checksum = %q, want %q", checksum, tt.wantChecksum)
			}
		})
	}
}
