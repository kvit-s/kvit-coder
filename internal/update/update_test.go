package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.4.0", "v0.4.0", 0},
		{"v0.4.0", "v0.5.0", -1},
		{"v0.5.0", "v0.4.0", 1},
		{"v0.4.0", "v0.4.1", -1},
		{"v1.0.0", "v0.9.9", 1},
		{"0.4.0", "v0.4.0", 0},       // missing v is fine
		{"v0.4", "v0.4.0", 0},        // short form pads with zeros
		{"v0.4.0-rc1", "v0.4.0", -1}, // prerelease is older than the release
		{"v0.4.0", "v0.4.0-rc1", 1},
		{"dev", "v0.1.0", -1}, // a dev build is older than any release
		{"dev", "dev", 0},
		// Commits past a tag are newer than the tag, not prereleases.
		{"v0.4.0-1-gc031548", "v0.4.0", 1},
		{"v0.4.0", "v0.4.0-1-gc031548", -1},
		{"v0.4.0-1-gc031548", "v0.4.0-2-gdeadbe", -1},
		{"v0.4.0-1-gc031548", "v0.4.0-rc1", 1},
		{"v0.4.0-1-gc031548-dirty", "v0.4.0", 1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
	if !NewerThan("v0.4.0", "v0.5.0") {
		t.Error("NewerThan(v0.4.0, v0.5.0) = false, want true")
	}
	if NewerThan("v0.5.0", "v0.4.0") {
		t.Error("NewerThan(v0.5.0, v0.4.0) = true, want false")
	}
}

func TestIsDevVersion(t *testing.T) {
	for _, v := range []string{"", "dev", "unknown", "v0.4.0-dirty", "v0.4.0-3-gabc-dirty"} {
		if !IsDevVersion(v) {
			t.Errorf("IsDevVersion(%q) = false, want true", v)
		}
	}
	for _, v := range []string{"v0.4.0", "0.4.0", "v0.4.0-rc1"} {
		if IsDevVersion(v) {
			t.Errorf("IsDevVersion(%q) = true, want false", v)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("v0.5.0", "linux", "amd64"); got != "kvit-coder_0.5.0_linux_amd64.tar.gz" {
		t.Errorf("linux asset = %q", got)
	}
	if got := AssetName("v0.5.0", "windows", "amd64"); got != "kvit-coder_0.5.0_windows_amd64.zip" {
		t.Errorf("windows asset = %q", got)
	}
	if got := AssetName("v0.5.0", "darwin", "arm64"); got != "kvit-coder_0.5.0_darwin_arm64.tar.gz" {
		t.Errorf("darwin asset = %q", got)
	}
	if got := NormalizeTag("0.5.0"); got != "v0.5.0" {
		t.Errorf("NormalizeTag(0.5.0) = %q", got)
	}
	if got := NormalizeTag("v0.5.0"); got != "v0.5.0" {
		t.Errorf("NormalizeTag(v0.5.0) = %q", got)
	}
}

func TestShouldAutoCheck(t *testing.T) {
	now := time.Now().UTC()
	day := 24 * time.Hour

	if ShouldAutoCheck(now, "v0.4.0", nil, true, day) != true {
		t.Error("no state should check")
	}
	fresh := &State{LastCheck: now.Add(-time.Hour), LatestTag: "v0.5.0"}
	if ShouldAutoCheck(now, "v0.4.0", fresh, true, day) {
		t.Error("fresh cache should not check")
	}
	stale := &State{LastCheck: now.Add(-25 * time.Hour), LatestTag: "v0.5.0"}
	if !ShouldAutoCheck(now, "v0.4.0", stale, true, day) {
		t.Error("stale cache should check")
	}
	if ShouldAutoCheck(now, "v0.4.0", stale, false, day) {
		t.Error("disabled config should not check")
	}
	if ShouldAutoCheck(now, "dev", nil, true, day) {
		t.Error("dev build should not auto-check")
	}
	if ShouldAutoCheck(now, "v0.4.0-dirty", nil, true, day) {
		t.Error("dirty build should not auto-check")
	}
	t.Setenv(NoCheckEnv, "1")
	if ShouldAutoCheck(now, "v0.4.0", nil, true, day) {
		t.Error("env opt-out should not check")
	}
}

func TestLatestTagRedirect(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/kvit-s/kvit-coder/releases/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "release page")
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	old := latestPageURL
	latestPageURL = srv.URL + "/latest"
	defer func() { latestPageURL = old }()

	tag, err := LatestTag(context.Background())
	if err != nil {
		t.Fatalf("LatestTag: %v", err)
	}
	if tag != "v9.9.9" {
		t.Fatalf("LatestTag = %q, want v9.9.9", tag)
	}
}

func TestStateRoundtrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	NoteCheck("v0.5.0")
	st := LoadState()
	if st == nil {
		t.Fatal("LoadState = nil after NoteCheck")
	}
	if st.LatestTag != "v0.5.0" {
		t.Fatalf("LatestTag = %q", st.LatestTag)
	}
	if time.Since(st.LastCheck) > time.Minute {
		t.Fatalf("LastCheck not recent: %v", st.LastCheck)
	}
	// A failed (offline) check keeps the tag but refreshes the time.
	NoteCheck("")
	if kept := LoadState(); kept == nil || kept.LatestTag != "v0.5.0" {
		t.Fatalf("empty NoteCheck lost the tag: %+v", kept)
	}
}

func writeSums(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	sums := hex.EncodeToString(sum[:]) + "  " + name + "\n" +
		"deadbeef  kvit-coder_0.0.0_other_os.tar.gz\n"
	sumsPath := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(sumsPath, []byte(sums), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyArchive(t *testing.T) {
	dir := t.TempDir()
	data := []byte("fake release bytes")
	archive := writeSums(t, dir, "kvit-coder_0.5.0_linux_amd64.tar.gz", data)
	if err := VerifyArchive(archive, filepath.Join(dir, "checksums.txt")); err != nil {
		t.Fatalf("VerifyArchive: %v", err)
	}
	// Corrupt the archive: the checksum must fail, not pass.
	if err := os.WriteFile(archive, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArchive(archive, filepath.Join(dir, "checksums.txt")); err == nil {
		t.Fatal("VerifyArchive passed on tampered archive")
	} else if !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("wrong error: %v", err)
	}
	// An archive the sums file never mentions is refused.
	other := filepath.Join(dir, "kvit-coder_9.9.9_linux_amd64.tar.gz")
	if err := os.WriteFile(other, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArchive(other, filepath.Join(dir, "checksums.txt")); err == nil {
		t.Fatal("VerifyArchive passed on unlisted archive")
	}
}

func makeTarGz(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "kvit-coder_0.5.0_linux_amd64.tar.gz")
	if err := os.WriteFile(p, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func makeZip(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	p := filepath.Join(dir, "kvit-coder_0.5.0_windows_amd64.zip")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestExtractBinaries(t *testing.T) {
	files := map[string]string{
		"kvit-coder":          "coder binary",
		"kvit-coder-ui":       "ui binary",
		"config.example.yaml": "docs are ignored",
	}
	tarball := makeTarGz(t, t.TempDir(), files)
	out := t.TempDir()
	coder, ui, err := ExtractBinaries(tarball, out)
	if err != nil {
		t.Fatalf("extract tar.gz: %v", err)
	}
	if data, _ := os.ReadFile(coder); string(data) != "coder binary" {
		t.Errorf("coder content = %q", data)
	}
	if data, _ := os.ReadFile(ui); string(data) != "ui binary" {
		t.Errorf("ui content = %q", data)
	}

	zipFiles := map[string]string{
		"kvit-coder.exe":    "coder exe",
		"kvit-coder-ui.exe": "ui exe",
		"README.md":         "docs are ignored",
	}
	zipPath := makeZip(t, t.TempDir(), zipFiles)
	out2 := t.TempDir()
	coder2, ui2, err := ExtractBinaries(zipPath, out2)
	if err != nil {
		t.Fatalf("extract zip: %v", err)
	}
	if data, _ := os.ReadFile(coder2); string(data) != "coder exe" {
		t.Errorf("coder exe content = %q", data)
	}
	if data, _ := os.ReadFile(ui2); string(data) != "ui exe" {
		t.Errorf("ui exe content = %q", data)
	}

	// An archive without binaries is refused.
	empty := makeTarGz(t, t.TempDir(), map[string]string{"README.md": "nope"})
	if _, _, err := ExtractBinaries(empty, t.TempDir()); err == nil {
		t.Error("extract of binary-less archive passed")
	}
}

func TestReplaceUnix(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("unix replace semantics")
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "kvit-coder")
	if err := os.WriteFile(dest, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "new-src")
	if err := os.WriteFile(src, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := replaceUnix(dest, src); err != nil {
		t.Fatalf("replaceUnix: %v", err)
	}
	if data, _ := os.ReadFile(dest); string(data) != "new" {
		t.Fatalf("dest = %q, want new", data)
	}
	if st, err := os.Stat(dest); err != nil || st.Mode().Perm() != 0755 {
		t.Fatalf("dest mode = %v, %v", st.Mode(), err)
	}
}

// TestFetchRelease serves a fake release (archive plus checksums) and runs
// the whole download-verify path against it.
func TestFetchRelease(t *testing.T) {
	work := t.TempDir()
	var archive []byte
	{
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for name, content := range map[string]string{"kvit-coder": "new coder", "kvit-coder-ui": "new ui"} {
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(content))}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		archive = buf.Bytes()
	}
	sum := sha256.Sum256(archive)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "checksums.txt") {
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), AssetName("v0.5.0", "linux", "amd64"))
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), AssetName("v0.5.0", "linux", "arm64"))
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), AssetName("v0.5.0", "windows", "amd64"))
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), AssetName("v0.5.0", "windows", "arm64"))
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), AssetName("v0.5.0", "darwin", "amd64"))
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), AssetName("v0.5.0", "darwin", "arm64"))
			return
		}
		if _, err := w.Write(archive); err != nil {
			t.Errorf("test server write: %v", err)
		}
	}))
	defer srv.Close()

	oldBase := downloadBaseURL
	downloadBaseURL = srv.URL
	defer func() { downloadBaseURL = oldBase }()

	// The asset name follows the test platform; serve the same bytes under
	// whichever name FetchRelease asks for.
	got, err := FetchRelease(context.Background(), "v0.5.0", work)
	if err != nil {
		t.Fatalf("FetchRelease: %v", err)
	}
	if data, _ := os.ReadFile(got); !bytes.Equal(data, archive) {
		t.Fatal("fetched archive differs")
	}
}

// TestInstallPair replaces both binaries in a fake install dir, the way
// :update does after extracting a release.
func TestInstallPair(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("unix install path")
	}
	binDir := t.TempDir()
	coderDest := filepath.Join(binDir, "kvit-coder")
	uiDest := filepath.Join(binDir, "kvit-coder-ui")
	for _, d := range []string{coderDest, uiDest} {
		if err := os.WriteFile(d, []byte("old"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	srcDir := t.TempDir()
	coderSrc := filepath.Join(srcDir, "kvit-coder")
	uiSrc := filepath.Join(srcDir, "kvit-coder-ui")
	for _, s := range []string{coderSrc, uiSrc} {
		if err := os.WriteFile(s, []byte("new"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Install(coderSrc, uiSrc, coderDest, uiDest); err != nil {
		t.Fatalf("Install: %v", err)
	}
	for _, d := range []string{coderDest, uiDest} {
		data, err := os.ReadFile(d)
		if err != nil || string(data) != "new" {
			t.Fatalf("%s = %q, %v", d, data, err)
		}
		if st, err := os.Stat(d); err != nil || st.Mode().Perm() != 0755 {
			t.Fatalf("%s mode = %v, %v", d, st.Mode(), err)
		}
	}
}
