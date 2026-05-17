package engine

import "testing"

func TestParseFedoraLatestVersion(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    int
		wantErr bool
	}{
		{
			name: "picks highest stable version",
			html: `<a href="40/">40/</a> <a href="41/">41/</a> <a href="42/">42/</a>`,
			want: 42,
		},
		{
			name: "single version",
			html: `<a href="39/">39/</a>`,
			want: 39,
		},
		{
			name: "skips versions >= 100 (pre-release sentinel)",
			html: `<a href="42/">42/</a> <a href="100/">100/</a> <a href="101/">101/</a>`,
			want: 42,
		},
		{
			name:    "no version directories",
			html:    `<a href="test/">test/</a> <a href="rawhide/">rawhide/</a>`,
			wantErr: true,
		},
		{
			name:    "empty listing",
			html:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFedoraLatestVersion(tt.html)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFedoraLatestVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseFedoraLatestVersion() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseFedoraImageFilename(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		fArch   string
		version int
		want    string
		wantErr bool
	}{
		{
			name: "finds x86_64 image",
			html: `<a href="Fedora-Cloud-Base-Generic.x86_64-42-1.1.qcow2">` +
				`Fedora-Cloud-Base-Generic.x86_64-42-1.1.qcow2</a>`,
			fArch:   "x86_64",
			version: 42,
			want:    "Fedora-Cloud-Base-Generic.x86_64-42-1.1.qcow2",
		},
		{
			name: "finds aarch64 image",
			html: `<a href="Fedora-Cloud-Base-Generic.aarch64-42-1.1.qcow2">` +
				`Fedora-Cloud-Base-Generic.aarch64-42-1.1.qcow2</a>`,
			fArch:   "aarch64",
			version: 42,
			want:    "Fedora-Cloud-Base-Generic.aarch64-42-1.1.qcow2",
		},
		{
			name: "multiple images — picks correct arch",
			html: `<a href="Fedora-Cloud-Base-Generic.x86_64-42-1.1.qcow2">x86_64</a>` +
				`<a href="Fedora-Cloud-Base-Generic.aarch64-42-1.1.qcow2">aarch64</a>`,
			fArch:   "aarch64",
			version: 42,
			want:    "Fedora-Cloud-Base-Generic.aarch64-42-1.1.qcow2",
		},
		{
			name: "version mismatch — not found",
			html: `<a href="Fedora-Cloud-Base-Generic.x86_64-41-1.1.qcow2">` +
				`Fedora-Cloud-Base-Generic.x86_64-41-1.1.qcow2</a>`,
			fArch:   "x86_64",
			version: 42,
			wantErr: true,
		},
		{
			name:    "empty listing",
			html:    "",
			fArch:   "x86_64",
			version: 42,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFedoraImageFilename(tt.html, tt.fArch, tt.version)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFedoraImageFilename() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("parseFedoraImageFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}
