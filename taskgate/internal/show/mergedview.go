package show

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ynny-github/taskgate/taskgate/internal/annotation"
)

// Audience is the caller's audience filter. taskgate show -> Human,
// taskgate ai show -> AI. The merged view always includes the "shared"
// bucket in addition to the audience's own bucket.
type Audience int

const (
	AudienceHuman Audience = iota
	AudienceAI
)

// Bucket returns the audience-specific bucket directory name. The shared
// bucket is constant ("shared") and is always merged in addition.
func (a Audience) Bucket() string {
	switch a {
	case AudienceAI:
		return "ai"
	default:
		return "human"
	}
}

// EntryKind discriminates tasks from directories.
type EntryKind int

const (
	EntryKindTask EntryKind = iota
	EntryKindDirectory
)

// Entry is a single row in the merged view.
type Entry struct {
	Name       string
	Path       string
	Kind       EntryKind
	Annotation annotation.AnnotationBlock
	// Note carries a non-fatal author-facing message produced during
	// resolution (e.g. "permission denied", "symlink escapes workspace").
	// Empty when the entry resolved cleanly. Surfaced on stderr by Run
	// in the human form; absent from the AI envelope (the AI client sees
	// summary: null instead).
	Note string
}

// ResolvedTarget is the outcome of a single-name lookup.
type ResolvedTarget struct {
	Kind     EntryKind
	Entry    Entry
	Children []Entry
}

// LessEntries implements the FR-007 sort: directories first, then tasks;
// within each group, basename case-sensitive lexicographic order.
func LessEntries(a, b Entry) bool {
	if a.Kind != b.Kind {
		return a.Kind == EntryKindDirectory
	}
	return a.Name < b.Name
}

// EntrySlice attaches sort.Interface to []Entry using LessEntries.
type EntrySlice []Entry

func (s EntrySlice) Len() int           { return len(s) }
func (s EntrySlice) Less(i, j int) bool { return LessEntries(s[i], s[j]) }
func (s EntrySlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// bucketRelPath is the project-relative path of a bucket directory,
// e.g. ".taskgate/human". It is what gets recorded on Entry.Path.
func bucketRelPath(bucket string, sub string) string {
	rel := filepath.Join(".taskgate", bucket)
	if sub != "" {
		rel = filepath.Join(rel, sub)
	}
	return filepath.ToSlash(rel)
}

// scanBucket reads one level inside <workspaceDir>/<bucket>/<sub>.
// Returns map keyed by basename pointing at the bucket-qualified Entry
// (so the caller can detect cross-bucket collisions and merge).
// Non-executable regular files are hidden (FR-014); an escaping/broken
// symlink (note != "") is left listed regardless of executable bit.
func scanBucket(workspaceDir, bucket, sub string) (map[string]Entry, error) {
	absDir := filepath.Join(workspaceDir, bucket, sub)
	dirEntries, err := os.ReadDir(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make(map[string]Entry, len(dirEntries))
	for _, de := range dirEntries {
		name := de.Name()
		absChild := filepath.Join(absDir, name)
		kind := EntryKindTask
		if de.IsDir() {
			kind = EntryKindDirectory
		}
		ann, note := loadAnnotationForWithNote(absChild, kind)
		if kind == EntryKindTask && note == "" && !isExecutable(absChild) {
			// Non-executable files cannot be run; hide them (FR-014).
			// An escaping/broken symlink (note != "") is left listed.
			continue
		}
		out[name] = Entry{
			Name:       name,
			Path:       bucketRelPath(bucket, filepath.Join(sub, name)),
			Kind:       kind,
			Annotation: ann,
			Note:       note,
		}
	}
	return out, nil
}

// loadAnnotationForWithNote returns a task file's annotation plus a
// non-fatal note describing any read failure. Directories carry no
// annotation. An escaping or broken symlink yields an empty block and a
// descriptive note (FR-008).
func loadAnnotationForWithNote(absPath string, kind EntryKind) (annotation.AnnotationBlock, string) {
	if note := symlinkEscapeNote(absPath); note != "" {
		return annotation.AnnotationBlock{}, note
	}
	if kind == EntryKindDirectory {
		return annotation.AnnotationBlock{}, ""
	}
	f, err := os.Open(absPath)
	if err != nil {
		if os.IsPermission(err) {
			return annotation.AnnotationBlock{}, "permission denied"
		}
		return annotation.AnnotationBlock{}, ""
	}
	defer f.Close()
	block, err := annotation.Parse(f)
	if err != nil {
		return annotation.AnnotationBlock{}, ""
	}
	return block, ""
}

// isExecutable reports whether absPath is an executable file. Symlinks are
// followed to their target's mode — the same file taskgate run executes.
func isExecutable(absPath string) bool {
	info, err := os.Stat(absPath)
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

// symlinkEscapeNote returns a note when absPath is a symlink whose
// target leaves the workspace root (FR-008). Empty when the entry is
// safe to read. Resolution failures are also treated as "safe": we'd
// rather list the entry than over-aggressively warn.
func symlinkEscapeNote(absPath string) string {
	info, err := os.Lstat(absPath)
	if err != nil {
		return ""
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "broken symlink"
	}
	workspaceRoot, err := workspaceRootFromAbs(absPath)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(workspaceRoot, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "symlink target escapes .taskgate/"
	}
	return ""
}

// workspaceRootFromAbs walks up from absPath until it finds the
// .taskgate/ ancestor. Returns its absolute path or an error if none.
func workspaceRootFromAbs(absPath string) (string, error) {
	dir := filepath.Dir(absPath)
	for {
		if filepath.Base(dir) == ".taskgate" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// mergeLevel combines two bucket scans into a single []Entry, or returns a
// CollisionReport if any basename appears in both. Audience entries are
// kept on collision-free hits exactly as they came in.
func mergeLevel(audience, shared map[string]Entry) ([]Entry, *CollisionReport) {
	// Detect collisions first; if any name is in both maps, return the
	// first one we find. (Any single collision is enough to abort the
	// level per FR-013, but we surface ALL conflicting paths for that one
	// name in the report.)
	for name, aEntry := range audience {
		if sEntry, ok := shared[name]; ok {
			return nil, &CollisionReport{
				Name:  name,
				Paths: []string{aEntry.Path, sEntry.Path},
			}
		}
	}
	merged := make([]Entry, 0, len(audience)+len(shared))
	for _, e := range audience {
		merged = append(merged, e)
	}
	for _, e := range shared {
		merged = append(merged, e)
	}
	sort.Sort(EntrySlice(merged))
	return merged, nil
}

// ValidateName checks that name is a run-style argument (bare or
// slash-separated) and not a filesystem path. Returns nil on success or
// a populated InvalidInputReport on failure.
func ValidateName(input string) *InvalidInputReport {
	if input == "" {
		return &InvalidInputReport{Input: input, Reason: "empty"}
	}
	if input[0] == '/' {
		return &InvalidInputReport{Input: input, Reason: "absolute_path"}
	}
	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") ||
		input == "." || input == ".." {
		return &InvalidInputReport{Input: input, Reason: "parent_escape"}
	}
	if strings.Contains(input, ".taskgate/") {
		return &InvalidInputReport{Input: input, Reason: "filesystem_path"}
	}
	return nil
}

// SearchedBuckets returns the bucket directory paths (project-relative)
// that show consulted for a given audience — used by NotFoundReport.
func SearchedBuckets(audience Audience) []string {
	return []string{
		bucketRelPath(audience.Bucket(), ""),
		bucketRelPath("shared", ""),
	}
}

// ResolveName walks the merged tree level-by-level and returns either a
// task target, a directory target, a CollisionReport, or a NotFoundReport.
// At each level it applies the same cross-bucket collision check used by
// ResolveRoot.
func ResolveName(audience Audience, workspaceDir, name string) (*ResolvedTarget, *CollisionReport, *NotFoundReport, error) {
	segments := strings.Split(name, "/")
	for _, seg := range segments {
		if seg == "" || seg == "." || seg == ".." {
			return nil, nil, &NotFoundReport{Name: name, Searched: SearchedBuckets(audience)}, nil
		}
	}

	var current *Entry
	var subPath string
	for i, seg := range segments {
		aud, err := scanBucketSegment(workspaceDir, audience.Bucket(), subPath, seg)
		if err != nil {
			return nil, nil, nil, err
		}
		sh, err := scanBucketSegment(workspaceDir, "shared", subPath, seg)
		if err != nil {
			return nil, nil, nil, err
		}
		if aud != nil && sh != nil {
			return nil, &CollisionReport{Name: joinName(segments[:i+1]), Paths: []string{aud.Path, sh.Path}}, nil, nil
		}
		var hit *Entry
		switch {
		case aud != nil:
			hit = aud
		case sh != nil:
			hit = sh
		default:
			return nil, nil, &NotFoundReport{Name: name, Searched: SearchedBuckets(audience)}, nil
		}
		// All non-final segments must be directories; if a non-final hit
		// is a task we treat it as not found per FR-010 (no descent into
		// a task file).
		if i < len(segments)-1 && hit.Kind != EntryKindDirectory {
			return nil, nil, &NotFoundReport{Name: name, Searched: SearchedBuckets(audience)}, nil
		}
		current = hit
		if subPath == "" {
			subPath = seg
		} else {
			subPath = subPath + "/" + seg
		}
	}

	if current == nil {
		return nil, nil, &NotFoundReport{Name: name, Searched: SearchedBuckets(audience)}, nil
	}

	if current.Kind == EntryKindTask {
		return &ResolvedTarget{Kind: EntryKindTask, Entry: *current}, nil, nil, nil
	}

	// Directory target — load children + collision-check at the children level.
	children, col, err := resolveDirectoryChildren(audience, workspaceDir, subPath)
	if err != nil {
		return nil, nil, nil, err
	}
	if col != nil {
		return nil, col, nil, nil
	}
	return &ResolvedTarget{Kind: EntryKindDirectory, Entry: *current, Children: children}, nil, nil, nil
}

// scanBucketSegment returns an Entry for <bucket>/<sub>/<seg> if it
// exists, else nil. Non-executable regular files return nil (FR-014).
func scanBucketSegment(workspaceDir, bucket, sub, seg string) (*Entry, error) {
	abs := filepath.Join(workspaceDir, bucket, sub, seg)
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	kind := EntryKindTask
	if info.IsDir() {
		kind = EntryKindDirectory
	}
	ann, note := loadAnnotationForWithNote(abs, kind)
	if kind == EntryKindTask && note == "" && !isExecutable(abs) {
		return nil, nil // non-executable file: not resolvable (FR-014)
	}
	return &Entry{
		Name:       seg,
		Path:       bucketRelPath(bucket, filepath.Join(sub, seg)),
		Kind:       kind,
		Annotation: ann,
		Note:       note,
	}, nil
}

// resolveDirectoryChildren scans the immediate children of a resolved
// directory across both audience and shared buckets, detects collisions,
// and returns a sorted Entry slice.
func resolveDirectoryChildren(audience Audience, workspaceDir, sub string) ([]Entry, *CollisionReport, error) {
	aud, err := scanBucket(workspaceDir, audience.Bucket(), sub)
	if err != nil {
		return nil, nil, err
	}
	sh, err := scanBucket(workspaceDir, "shared", sub)
	if err != nil {
		return nil, nil, err
	}
	merged, col := mergeLevel(aud, sh)
	if col != nil {
		return nil, col, nil
	}
	return merged, nil, nil
}

func joinName(segs []string) string {
	return strings.Join(segs, "/")
}

// ResolveRoot returns the merged-view entries at the root of the workspace.
// On collision returns (nil, report, nil). On read error returns (nil, nil, err).
func ResolveRoot(audience Audience, workspaceDir string) ([]Entry, *CollisionReport, error) {
	aud, err := scanBucket(workspaceDir, audience.Bucket(), "")
	if err != nil {
		return nil, nil, err
	}
	sh, err := scanBucket(workspaceDir, "shared", "")
	if err != nil {
		return nil, nil, err
	}
	merged, col := mergeLevel(aud, sh)
	if col != nil {
		return nil, col, nil
	}
	return merged, nil, nil
}

// WorkspaceDir returns the absolute path of .taskgate/ inside cwd or
// ErrWorkspaceMissing when it does not exist.
func WorkspaceDir(cwd string) (string, error) {
	ws := filepath.Join(cwd, ".taskgate")
	info, err := os.Stat(ws)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrWorkspaceMissing
		}
		return "", err
	}
	if !info.IsDir() {
		return "", ErrWorkspaceMissing
	}
	return ws, nil
}
