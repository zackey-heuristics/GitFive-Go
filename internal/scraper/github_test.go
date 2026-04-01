package scraper

import "testing"

func TestParseBlobURL(t *testing.T) {
	tests := []struct {
		url                                        string
		wantUser, wantRepo, wantCommit, wantFile   string
		wantErr                                    bool
	}{
		{
			"https://github.com/alice/myrepo/blob/abc123def456abc123def456abc123def456abcd/path/to/file.go",
			"alice", "myrepo", "abc123def456abc123def456abc123def456abcd", "path/to/file.go",
			false,
		},
		{
			"not-a-blob-url",
			"", "", "", "",
			true,
		},
	}
	for _, tt := range tests {
		user, repo, commit, file, err := ParseBlobURL(tt.url)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseBlobURL(%q): err=%v, wantErr=%v", tt.url, err, tt.wantErr)
			continue
		}
		if user != tt.wantUser || repo != tt.wantRepo || commit != tt.wantCommit || file != tt.wantFile {
			t.Errorf("ParseBlobURL(%q) = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
				tt.url, user, repo, commit, file,
				tt.wantUser, tt.wantRepo, tt.wantCommit, tt.wantFile)
		}
	}
}
