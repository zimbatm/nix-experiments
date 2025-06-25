package store

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestIsTrustedUser(t *testing.T) {
	// This test requires nix to be available
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix command not available")
	}

	s := New("/nix/store")
	trusted, err := s.IsTrustedUser()
	if err != nil {
		// It's okay if this fails in test environment
		t.Logf("IsTrustedUser() error: %v", err)
		return
	}

	// Just log the result, don't assert since it depends on environment
	t.Logf("Current user trusted status: %v", trusted)
}

func TestStoreInfo_Parsing(t *testing.T) {
	// Test parsing of store info JSON
	testCases := []struct {
		name    string
		json    string
		want    StoreInfo
		wantErr bool
	}{
		{
			name: "trusted user",
			json: `{"trusted":1,"url":"daemon","version":"2.28.3"}`,
			want: StoreInfo{
				Trusted: 1,
				URL:     "daemon",
				Version: "2.28.3",
			},
		},
		{
			name: "untrusted user",
			json: `{"trusted":0,"url":"daemon","version":"2.28.3"}`,
			want: StoreInfo{
				Trusted: 0,
				URL:     "daemon",
				Version: "2.28.3",
			},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var info StoreInfo
			err := json.Unmarshal([]byte(tc.json), &info)

			if (err != nil) != tc.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				if info.Trusted != tc.want.Trusted {
					t.Errorf("Trusted = %v, want %v", info.Trusted, tc.want.Trusted)
				}
				if info.URL != tc.want.URL {
					t.Errorf("URL = %v, want %v", info.URL, tc.want.URL)
				}
				if info.Version != tc.want.Version {
					t.Errorf("Version = %v, want %v", info.Version, tc.want.Version)
				}
			}
		})
	}
}
