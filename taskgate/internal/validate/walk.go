package validate

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

const indexFilename = "_index"

// bucketDisplayPath returns the .taskgate-relative slash path for an entry,
// matching internal/show's convention.
func bucketDisplayPath(bucket, rel string) string {
	p := filepath.Join(".taskgate", bucket, filepath.FromSlash(rel))
	return filepath.ToSlash(p)
}

// discoverBucket recursively walks <workspaceDir>/<bucket>. It returns every
// checkable file and the set of collision slots (task-file and subdirectory
// rels — never _index). A missing bucket directory yields empty results.
func discoverBucket(workspaceDir, bucket string) ([]discovered, map[string]string, error) {
	var files []discovered
	slots := map[string]string{}

	var walk func(sub string) error
	walk = func(sub string) error {
		absDir := filepath.Join(workspaceDir, bucket, filepath.FromSlash(sub))
		entries, err := os.ReadDir(absDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, de := range entries {
			name := de.Name()
			if strings.HasPrefix(name, ".") {
				continue // skip .gitkeep and other dotfiles
			}
			rel := name
			if sub != "" {
				rel = path.Join(sub, name)
			}
			abs := filepath.Join(absDir, name)
			display := bucketDisplayPath(bucket, rel)

			if de.IsDir() {
				slots[rel] = display
				if err := walk(rel); err != nil {
					return err
				}
				continue
			}
			if name == indexFilename {
				files = append(files, discovered{
					absPath:     abs,
					displayPath: display,
					logicalName: sub, // the directory this _index describes
					isIndex:     true,
				})
				continue
			}
			slots[rel] = display
			files = append(files, discovered{
				absPath:     abs,
				displayPath: display,
				logicalName: rel,
				isIndex:     false,
			})
		}
		return nil
	}

	if err := walk(""); err != nil {
		return nil, nil, err
	}
	return files, slots, nil
}

// detectCollisions reports a name that occupies the same slot in the shared
// bucket and an audience bucket. Two collision spaces are checked: shared×human
// and shared×ai. human×ai is intentionally not a collision (FR-013).
func detectCollisions(slots map[string]map[string]string) []Finding {
	var out []Finding
	shared := slots["shared"]
	for _, aud := range []string{"human", "ai"} {
		audSlots := slots[aud]
		for rel, sharedDisplay := range shared {
			if audDisplay, ok := audSlots[rel]; ok {
				out = append(out, Finding{
					Rule:    RuleCollision,
					Name:    rel,
					Paths:   []string{sharedDisplay, audDisplay},
					logical: rel,
				})
			}
		}
	}
	return out
}
