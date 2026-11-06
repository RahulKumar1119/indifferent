package storage

import "testing"

func TestProjectLogoKey_IsTheOneKeyTheUploadRouteHandsOut(t *testing.T) {
	if got, want := ProjectLogoKey("u1", "p1"), "uploads/u1/p1/logo.png"; got != want {
		t.Fatalf("ProjectLogoKey = %q, want %q (the API's upload route and every reader must agree)", got, want)
	}
}

func TestIsProjectLogoKey(t *testing.T) {
	cases := []struct {
		name       string
		user, proj string
		key        string
		want       bool
	}{
		{"its own logo", "u1", "p1", "uploads/u1/p1/logo.png", true},
		{"another user's logo", "u1", "p1", "uploads/u2/p1/logo.png", false},
		{"another project's logo", "u1", "p1", "uploads/u1/p2/logo.png", false},
		{"a rendered video", "u1", "p1", "output/p2/video.mp4", false},
		{"another user's source file", "u1", "p1", "uploads/u2/job9/source.mp4", false},
		{"the project's own text file", "u1", "p1", "uploads/u1/p1/input.txt", false},
		{"a different file name", "u1", "p1", "uploads/u1/p1/logo.png.bak", false},
		{"path traversal", "u1", "p1", "uploads/u1/p1/../../u2/p9/logo.png", false},
		{"traversal to the same name", "u1", "p1", "uploads/u1/p1/./logo.png", false},
		{"different case", "u1", "p1", "Uploads/u1/p1/logo.png", false},
		{"trailing slash", "u1", "p1", "uploads/u1/p1/logo.png/", false},
		{"empty key", "u1", "p1", "", false},
		{"empty user never matches", "", "p1", "uploads//p1/logo.png", false},
		{"empty project never matches", "u1", "", "uploads/u1//logo.png", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsProjectLogoKey(c.user, c.proj, c.key); got != c.want {
				t.Errorf("IsProjectLogoKey(%q, %q, %q) = %v, want %v", c.user, c.proj, c.key, got, c.want)
			}
		})
	}
}

func TestLogoKeyBelongsToProject(t *testing.T) {
	cases := []struct {
		name string
		proj string
		key  string
		want bool
	}{
		{"the project's own logo, any user segment", "p1", "uploads/u1/p1/logo.png", true},
		{"a Google numeric user id", "p1", "uploads/113047522329917702092/p1/logo.png", true},
		{"another project's logo", "p1", "uploads/u1/p2/logo.png", false},
		{"a rendered video", "p1", "output/p1/video.mp4", false},
		{"an input file of the same project", "p1", "uploads/u1/p1/input.txt", false},
		{"too few segments", "p1", "uploads/p1/logo.png", false},
		{"too many segments", "p1", "uploads/u1/x/p1/logo.png", false},
		{"traversal in the user segment", "p1", "uploads/../p1/logo.png", false},
		{"dot in the user segment", "p1", "uploads/./p1/logo.png", false},
		{"empty user segment", "p1", "uploads//p1/logo.png", false},
		{"wrong first segment", "p1", "temp/u1/p1/logo.png", false},
		{"empty project never matches", "", "uploads/u1//logo.png", false},
		{"empty key", "p1", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LogoKeyBelongsToProject(c.proj, c.key); got != c.want {
				t.Errorf("LogoKeyBelongsToProject(%q, %q) = %v, want %v", c.proj, c.key, got, c.want)
			}
		})
	}
}
