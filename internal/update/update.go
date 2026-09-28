// Package update checks the GitHub releases for a newer build and installs
// it. It is used only by kvit-coder-ui: the headless agent never checks, and
// the model is never offered it as a tool.
//
// Releases are built by GoReleaser on v* tags with predictable asset names
// (kvit-coder_<number>_<os>_<arch>.tar.gz, zip on Windows) plus a
// checksums.txt, so the check needs no API token: the /releases/latest
// redirect names the tag, and the archive is verified against checksums.txt
// before anything is replaced.
//
// On unix the new binaries are renamed over the old ones atomically. A
// running Windows .exe cannot be overwritten, but it can be renamed, so there
// the current file is renamed aside to .old and the staged .new takes its
// name; the .old is removed on the next start. No launcher shim or versioned
// directories are needed on either platform.
package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Repo is the GitHub repository releases are read from.
const Repo = "kvit-s/kvit-coder"

// NoCheckEnv disables the automatic check when set to 1/true/yes/on.
const NoCheckEnv = "KVIT_NO_UPDATE_CHECK"

// DefaultInterval is the automatic check cadence when the config names none.
const DefaultInterval = 24 * time.Hour

const stateFileName = "update-check.json"

// latestPageURL names the newest release through its redirect, which avoids
// the API and its rate limits. downloadBaseURL serves the release files. Both
// are variables so tests can point them at a local server.
var latestPageURL = "https://github.com/" + Repo + "/releases/latest"

var downloadBaseURL = "https://github.com/" + Repo + "/releases/download"

// State is the on-disk record of the last check, kept so the UI checks at
// most once per interval.
type State struct {
	LastCheck time.Time `json:"last_check"`
	LatestTag string    `json:"latest_tag"`
}

// Result is one version check.
type Result struct {
	Current    string
	Latest     string
	Available  bool
	ReleaseURL string
}

// ReleaseURL is the human page for a tag, where the notes live.
func ReleaseURL(tag string) string {
	return "https://github.com/" + Repo + "/releases/tag/" + tag
}

// IsDevVersion reports whether a stamped version is a development build,
// which is never auto-checked: "dev" from a plain go build, or a -dirty tree.
func IsDevVersion(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || v == "dev" || v == "unknown" || strings.Contains(v, "dirty")
}

// Compare orders two versions: -1 when a is older, 0 when equal, +1 when a
// is newer. Tags carry a leading v; normalization strips it, compares three
// numeric parts, and orders a bare release above its prereleases. Anything
// unparsable compares as zeros, so "dev" is older than any release.
//
// A git-describe suffix (v0.4.0-1-gSHA, what git describe prints for commits
// past a tag) is newer than the tag itself, not a prerelease: it names real
// commits on top. Two such suffixes order by commit count.
func Compare(a, b string) int {
	ca, pa := splitVersion(a)
	cb, pb := splitVersion(b)
	for i := 0; i < 3; i++ {
		if ca[i] != cb[i] {
			if ca[i] < cb[i] {
				return -1
			}
			return 1
		}
	}
	an, aDesc := describeCommits(pa)
	bn, bDesc := describeCommits(pb)
	switch {
	case aDesc && bDesc:
		switch {
		case an != bn:
			if an < bn {
				return -1
			}
			return 1
		default:
			return strings.Compare(pa, pb)
		}
	case aDesc:
		// Commits past the tag are newer than the tag and anything that
		// sorts below it (prereleases, a dirty tree on the tag).
		return 1
	case bDesc:
		return -1
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	default:
		return strings.Compare(pa, pb)
	}
}

// NewerThan reports whether latest is actually newer than current.
func NewerThan(current, latest string) bool {
	return Compare(current, latest) < 0
}

func splitVersion(s string) ([3]int, string) {
	var out [3]int
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	pre := ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre, s = s[i+1:], s[:i]
	}
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		if n < 0 {
			n = 0
		}
		out[i] = n
	}
	return out, pre
}

// describeCommits reports whether a version suffix is a git-describe
// commits-past-the-tag marker (N-gSHA, with an optional -dirty tail from a
// dirty tree) and how many commits past it names.
func describeCommits(pre string) (int, bool) {
	rest := strings.TrimSuffix(pre, "-dirty")
	idx := strings.Index(rest, "-g")
	if idx <= 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:idx])
	if err != nil || n < 0 {
		return 0, false
	}
	sha := rest[idx+2:]
	if sha == "" {
		return 0, false
	}
	for _, c := range sha {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return 0, false
		}
	}
	return n, true
}

// NormalizeTag accepts what a person types for :update ("0.5.0" or "v0.5.0")
// and returns the release tag form.
func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" || strings.HasPrefix(tag, "v") || strings.HasPrefix(tag, "V") {
		return tag
	}
	return "v" + tag
}

// DisabledByEnv reports whether the environment turns the automatic check off.
func DisabledByEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(NoCheckEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// StatePath is where the check record lives.
func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kvit-coder", stateFileName), nil
}

// LoadState reads the check record. Absent, unreadable or corrupt means no
// record, not an error: the caller checks now.
func LoadState() *State {
	p, err := StatePath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil
	}
	return &st
}

// NoteCheck records that a check just ran. An empty tag keeps the previous
// one, so even a failed (offline) attempt throttles the next one to the next
// interval instead of retrying on every start.
func NoteCheck(tag string) {
	p, err := StatePath()
	if err != nil {
		return
	}
	st := LoadState()
	if st == nil {
		st = &State{}
	}
	st.LastCheck = time.Now().UTC()
	if tag != "" {
		st.LatestTag = tag
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	_ = os.WriteFile(p, data, 0600)
}

// ShouldAutoCheck reports whether a background check is due. The first run
// (no record) checks; afterwards the interval gates it.
func ShouldAutoCheck(now time.Time, current string, st *State, enabled bool, interval time.Duration) bool {
	if !enabled || DisabledByEnv() || IsDevVersion(current) {
		return false
	}
	if interval <= 0 {
		interval = DefaultInterval
	}
	if st == nil || st.LatestTag == "" || st.LastCheck.IsZero() {
		return true
	}
	return now.Sub(st.LastCheck) >= interval
}

// LatestTag asks GitHub which release is newest by following the
// /releases/latest redirect to /releases/tag/<tag> and reading the tag.
func LatestTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestPageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "kvit-coder-update-check")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("update check: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("update check: unexpected status %s", resp.Status)
	}
	tag := path.Base(strings.TrimSuffix(resp.Request.URL.Path, "/"))
	if tag == "" || tag == "latest" || !strings.HasPrefix(strings.ToLower(tag), "v") {
		return "", fmt.Errorf("update check: cannot tell the latest release from %s", resp.Request.URL.Redacted())
	}
	return tag, nil
}

// Check resolves the latest release and compares it to the running build.
func Check(ctx context.Context, current string) (Result, error) {
	latest, err := LatestTag(ctx)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Current:    current,
		Latest:     latest,
		Available:  NewerThan(current, latest),
		ReleaseURL: ReleaseURL(latest),
	}, nil
}

// BinaryNames are the two files a release archive holds for an OS.
func BinaryNames(goos string) []string {
	if goos == "windows" {
		return []string{"kvit-coder.exe", "kvit-coder-ui.exe"}
	}
	return []string{"kvit-coder", "kvit-coder-ui"}
}

// AssetName is the release file for a tag and platform, following GoReleaser's
// convention: the tag's number without the v.
func AssetName(tag, goos, goarch string) string {
	number := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(tag), "v"), "V")
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("kvit-coder_%s_%s_%s.%s", number, goos, goarch, ext)
}

// AssetURL serves one release file.
func AssetURL(tag, goos, goarch string) string {
	return downloadBaseURL + "/" + tag + "/" + AssetName(tag, goos, goarch)
}

// ChecksumsURL serves the release's SHA256 list.
func ChecksumsURL(tag string) string {
	return downloadBaseURL + "/" + tag + "/checksums.txt"
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "kvit-coder-update-check")
	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("download %s: status %s", url, resp.Status)
	}
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("download %s: %w", url, closeErr)
	}
	return nil
}

// FetchRelease downloads a release's archive and checksums into dir, verifies
// the archive, and returns its path.
func FetchRelease(ctx context.Context, tag, dir string) (string, error) {
	asset := AssetName(tag, runtime.GOOS, runtime.GOARCH)
	archivePath := filepath.Join(dir, asset)
	if err := downloadFile(ctx, AssetURL(tag, runtime.GOOS, runtime.GOARCH), archivePath); err != nil {
		return "", err
	}
	sumsPath := filepath.Join(dir, "checksums.txt")
	if err := downloadFile(ctx, ChecksumsURL(tag), sumsPath); err != nil {
		os.Remove(archivePath)
		return "", err
	}
	if err := VerifyArchive(archivePath, sumsPath); err != nil {
		return "", err
	}
	return archivePath, nil
}

// VerifyArchive checks the archive against its checksums.txt entry. The sums
// file lists every platform's archive, so only the downloaded file's line is
// compared.
func VerifyArchive(archivePath, sumsPath string) error {
	data, err := os.ReadFile(sumsPath)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	base := filepath.Base(archivePath)
	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := path.Base(strings.TrimPrefix(fields[1], "*"))
		if name == base {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("checksums.txt has no entry for %s; not installing", base)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(sum.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s; not installing", base)
	}
	return nil
}

// slotFor maps an archive entry to the binary it carries, accepting either
// spelling so a zip and a tarball share the matcher.
func slotFor(base string) string {
	switch strings.TrimSuffix(base, ".exe") {
	case "kvit-coder":
		return "coder"
	case "kvit-coder-ui":
		return "ui"
	}
	return ""
}

// ExtractBinaries unpacks the two binaries from a release archive into
// destDir (flat, ignoring the docs the archive also carries) and returns
// their paths.
func ExtractBinaries(archivePath, destDir string) (coderPath, uiPath string, err error) {
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return extractZip(archivePath, destDir)
	}
	return extractTarGz(archivePath, destDir)
}

func writeEntry(destDir, base string, mode os.FileMode, open func() (io.Reader, error)) (string, error) {
	outPath := filepath.Join(destDir, base)
	in, err := open()
	if err != nil {
		return "", err
	}
	out, err := os.OpenFile(outPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return outPath, nil
}

func extractZip(archivePath, destDir string) (string, string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", "", err
	}
	defer r.Close()
	found := map[string]string{}
	for _, f := range r.File {
		base := path.Base(f.Name)
		slot := slotFor(base)
		if slot == "" || found[slot] != "" {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", "", err
		}
		out, err := writeEntry(destDir, base, 0755, func() (io.Reader, error) { return rc, nil })
		rc.Close()
		if err != nil {
			return "", "", err
		}
		found[slot] = out
	}
	return checkedPair(found)
}

func extractTarGz(archivePath, destDir string) (string, string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", "", err
	}
	defer gz.Close()
	found := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := path.Base(hdr.Name)
		slot := slotFor(base)
		if slot == "" || found[slot] != "" {
			continue
		}
		// Entries are flat names; refuse anything that would escape destDir.
		if base != filepath.Base(base) || hdr.Name != base && path.Base(hdr.Name) != base {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, 256<<20))
		if err != nil {
			return "", "", err
		}
		out, err := writeEntry(destDir, base, 0755, func() (io.Reader, error) {
			return strings.NewReader(string(data)), nil
		})
		if err != nil {
			return "", "", err
		}
		found[slot] = out
	}
	return checkedPair(found)
}

func checkedPair(found map[string]string) (string, string, error) {
	if found["coder"] == "" || found["ui"] == "" {
		return "", "", fmt.Errorf("archive holds no kvit-coder binaries; not installing")
	}
	return found["coder"], found["ui"], nil
}

// Resolve returns the real file behind a path: bare names go through the
// PATH, symlinks (the ~/.local/bin farm scripts/install.sh builds) resolve
// to the checkout, and a missing file falls back to the absolute path.
func Resolve(p string) string {
	if p == "" {
		return ""
	}
	if !strings.ContainsAny(p, `/\`) {
		if r, err := exec.LookPath(p); err == nil {
			p = r
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}

// Destinations returns where the two running binaries live: the UI behind
// uiExe (empty means this process) and the agent behind agentPath.
func Destinations(uiExe, agentPath string) (uiDest, coderDest string, err error) {
	if uiExe == "" {
		var exeErr error
		uiExe, exeErr = os.Executable()
		if exeErr != nil {
			return "", "", fmt.Errorf("cannot locate the running binary: %w", exeErr)
		}
	}
	uiDest = Resolve(uiExe)
	if uiDest == "" {
		return "", "", fmt.Errorf("cannot locate the running binary")
	}
	if agentPath == "" {
		agentPath = "kvit-coder"
	}
	coderDest = Resolve(agentPath)
	if coderDest == "" {
		return "", "", fmt.Errorf("cannot locate the agent binary %q", agentPath)
	}
	for _, d := range []string{uiDest, coderDest} {
		st, err := os.Stat(d)
		if err != nil || st.IsDir() {
			return "", "", fmt.Errorf("binary not found: %s", d)
		}
	}
	return uiDest, coderDest, nil
}

// Pending names updates that were staged (.new) but could not be swapped into
// place, as new-file/destination pairs.
type Pending struct {
	Pairs [][2]string
}

func (p *Pending) Error() string {
	return fmt.Sprintf("%d staged update file(s) still need moving into place", len(p.Pairs))
}

// Install replaces both binaries with freshly extracted ones. On unix each
// replacement is a copy into the destination directory followed by an atomic
// rename, so a crash never leaves half a binary. On Windows the running files
// cannot be overwritten, but renaming them aside is allowed, so the current
// files move to .old and the staged .new files take their names; the .old
// files are removed on the next start. When the swap cannot happen the staged
// .new files are left and a *Pending explains the manual move.
func Install(coderSrc, uiSrc, coderDest, uiDest string) error {
	pairs := [][2]string{{coderSrc, coderDest}, {uiSrc, uiDest}}
	if runtime.GOOS == "windows" {
		return installWindows(pairs)
	}
	for _, pr := range pairs {
		if err := replaceUnix(pr[1], pr[0]); err != nil {
			return fmt.Errorf("install %s: %w", pr[1], err)
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func replaceUnix(dest, src string) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".kvit-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	tmp.Close()
	// The temp file is only a unique name: copyFile truncates it, and the
	// mode is set explicitly because CreateTemp files start at 0600.
	if err := copyFile(src, tmpName, 0755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0755); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func installWindows(pairs [][2]string) error {
	// Stage both first, so a failed download never leaves one new binary.
	for _, pr := range pairs {
		if err := copyFile(pr[0], pr[1]+".new", 0600); err != nil {
			return fmt.Errorf("stage %s: %w", pr[1]+".new", err)
		}
	}
	var pending [][2]string
	for _, pr := range pairs {
		dest := pr[1]
		if err := swapWindows(dest); err != nil {
			pending = append(pending, [2]string{dest + ".new", dest})
		}
	}
	if len(pending) > 0 {
		return &Pending{Pairs: pending}
	}
	return nil
}

// swapWindows renames the running file aside and moves the staged .new into
// its place. The process keeps running from the renamed .old handle; the next
// start loads the new file.
func swapWindows(dest string) error {
	oldFile, newFile := dest+".old", dest+".new"
	_ = os.Remove(oldFile)
	if err := os.Rename(dest, oldFile); err != nil {
		return err
	}
	if err := os.Rename(newFile, dest); err != nil {
		_ = os.Rename(oldFile, dest)
		return err
	}
	return nil
}

// CleanupBackups removes leftover .old files from a previous Windows swap.
// Best effort: errors are ignored.
func CleanupBackups(dirs ...string) {
	for _, dir := range dirs {
		for _, base := range []string{"kvit-coder.old", "kvit-coder-ui.old", "kvit-coder.exe.old", "kvit-coder-ui.exe.old"} {
			_ = os.Remove(filepath.Join(dir, base))
		}
	}
}

// PendingStaged lists staged .new files waiting for a manual move.
func PendingStaged(dirs ...string) []string {
	var out []string
	for _, dir := range dirs {
		for _, base := range []string{"kvit-coder.new", "kvit-coder-ui.new", "kvit-coder.exe.new", "kvit-coder-ui.exe.new"} {
			p := filepath.Join(dir, base)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				out = append(out, p)
			}
		}
	}
	return out
}

// CleanupForPaths removes leftover backups beside the UI and agent binaries.
func CleanupForPaths(paths ...string) {
	seen := map[string]bool{}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, exe)
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		dir := filepath.Dir(Resolve(p))
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		CleanupBackups(dir)
	}
}
