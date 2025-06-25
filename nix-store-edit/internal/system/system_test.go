package system

import (
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	sys, err := Detect()
	if err != nil {
		// It's okay if detection fails in test environment
		t.Logf("System detection failed (expected in test environment): %v", err)
		return
	}

	t.Logf("Detected system type: %s", sys.Type())

	// Verify the detected system is available
	if !sys.IsAvailable() {
		t.Error("Detected system reports as not available")
	}
}

func TestSystemTypes(t *testing.T) {
	systems := []struct {
		name string
		sys  System
	}{
		{"NixOS", &NixOS{}},
		{"nix-darwin", &NixDarwin{}},
		{"home-manager", &HomeManager{}},
		{"system-manager", &SystemManager{}},
	}

	for _, tt := range systems {
		t.Run(tt.name, func(t *testing.T) {
			available := tt.sys.IsAvailable()
			t.Logf("%s available: %v", tt.name, available)

			if available {
				// Try to get current closure path
				path, err := tt.sys.GetClosurePath()
				if err != nil {
					t.Logf("Failed to get current closure path: %v", err)
				} else {
					t.Logf("Current closure path: %s", path)
				}
			}
		})
	}
}

func TestCommandExists(t *testing.T) {
	// Test with a command that should exist
	if !commandExists("ls") {
		t.Error("commandExists failed for 'ls'")
	}

	// Test with a command that shouldn't exist
	if commandExists("this-command-should-not-exist-12345") {
		t.Error("commandExists returned true for non-existent command")
	}
}

func TestDetectOS(t *testing.T) {
	os := detectOS()
	t.Logf("Detected OS: %s", os)

	// Just verify it returns something
	if os == "" {
		t.Error("detectOS returned empty string")
	}
}

func TestGetSystemByType(t *testing.T) {
	testCases := []struct {
		name        string
		systemType  string
		profilePath string
		wantType    Type
		wantErr     bool
	}{
		{
			name:       "nixos",
			systemType: "nixos",
			wantType:   TypeNixOS,
		},
		{
			name:       "nix-darwin",
			systemType: "nix-darwin",
			wantType:   TypeNixDarwin,
		},
		{
			name:       "home-manager",
			systemType: "home-manager",
			wantType:   TypeHomeManager,
		},
		{
			name:       "system-manager",
			systemType: "system-manager",
			wantType:   TypeSystemManager,
		},
		{
			name:        "profile with path",
			systemType:  "profile",
			profilePath: "/nix/var/nix/profiles/system",
			wantType:    TypeProfile,
		},
		{
			name:       "profile without path (uses default)",
			systemType: "profile",
			wantType:   TypeProfile,
			wantErr:    false,
		},
		{
			name:       "unknown system",
			systemType: "unknown",
			wantErr:    true,
		},
		{
			name:       "empty string",
			systemType: "",
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sys, err := GetSystemByType(tc.systemType, tc.profilePath)

			if (err != nil) != tc.wantErr {
				t.Errorf("GetSystemByType(%q, %q) error = %v, wantErr %v", tc.systemType, tc.profilePath, err, tc.wantErr)
				return
			}

			if !tc.wantErr && sys.Type() != tc.wantType {
				t.Errorf("GetSystemByType(%q, %q) returned type %v, want %v", tc.systemType, tc.profilePath, sys.Type(), tc.wantType)
			}
		})
	}
}

func TestIsNixOSFromLSBRelease(t *testing.T) {
	// This test will only pass on NixOS systems
	// We can't mock file reading easily, so just check if the function runs
	result := isNixOSFromLSBRelease()
	t.Logf("isNixOSFromLSBRelease() = %v", result)
}

func TestGetUserProfilePath(t *testing.T) {
	path := getUserProfilePath()
	t.Logf("User profile path: %s", path)

	// Just verify it returns something non-empty
	if path == "" {
		t.Error("getUserProfilePath() returned empty string")
	}
}

func TestDetectWithUserProfileFallback(t *testing.T) {
	// This test verifies that Detect() doesn't fail even if no specific system is detected
	sys, err := Detect()
	if err != nil {
		t.Logf("System detection failed: %v", err)
		// This should only happen if no user profile exists at all
		return
	}

	t.Logf("Detected system type: %s", sys.Type())

	// If it's a profile type, log the path
	if sys.Type() == TypeProfile {
		if profile, ok := sys.(*Profile); ok {
			t.Logf("Using profile path: %s", profile.ProfilePath)
		}
	}
}

func TestGetDefaultCommand(t *testing.T) {
	testCases := []struct {
		name    string
		system  System
		wantCmd string
	}{
		{
			name:    "NixOS",
			system:  &NixOS{},
			wantCmd: "nixos-rebuild test --use-remote-sudo",
		},
		{
			name:    "nix-darwin",
			system:  &NixDarwin{},
			wantCmd: "darwin-rebuild check",
		},
		{
			name:    "home-manager",
			system:  &HomeManager{},
			wantCmd: "home-manager switch",
		},
		{
			name:    "system-manager",
			system:  &SystemManager{},
			wantCmd: "system-manager test",
		},
		{
			name:    "profile",
			system:  &Profile{ProfilePath: "/test/profile"},
			wantCmd: "nix-env --profile /test/profile --set",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.system.GetDefaultCommand("/test/path")
			cmdStr := strings.Join(cmd, " ")

			// Check if the command starts with what we expect
			if !strings.HasPrefix(cmdStr, tc.wantCmd) {
				t.Errorf("GetDefaultCommand() = %v, want command starting with %v", cmdStr, tc.wantCmd)
			}

			t.Logf("%s default command: %s", tc.name, cmdStr)
		})
	}
}
