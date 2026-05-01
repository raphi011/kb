// Command genassets fingerprints static assets so they can be served with a
// long, immutable Cache-Control without staleness when the content changes.
//
// For each input file in -dir it:
//  1. Computes a SHA-256 of the contents and takes the first 10 hex chars.
//  2. Removes any prior fingerprinted siblings (foo-<10hex>.ext, plus .gz).
//  3. Copies the source to -out/<base>-<hash>.<ext>.
//  4. Writes a gzip(9) sibling at <copy>.gz.
//  5. Records the logical → fingerprinted mapping in the manifest.
//
// The original source file is left untouched so a fresh `esbuild` step does
// not need to re-create it. The manifest is written to -manifest as JSON.
package main

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func main() {
	dir := flag.String("dir", "", "directory containing source assets")
	out := flag.String("out", "", "output directory for fingerprinted copies")
	manifestPath := flag.String("manifest", "", "output manifest.json path (typically inside -out)")
	filesCSV := flag.String("files", "", "comma-separated list of logical asset filenames to fingerprint")
	flag.Parse()

	if *dir == "" || *out == "" || *manifestPath == "" || *filesCSV == "" {
		flag.Usage()
		os.Exit(2)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("mkdir %s: %v", *out, err)
	}

	files := splitTrim(*filesCSV)
	manifest := make(map[string]string, len(files))

	for _, name := range files {
		hashed, err := fingerprint(*dir, *out, name)
		if err != nil {
			log.Fatalf("fingerprint %s: %v", name, err)
		}
		manifest[name] = hashed
	}

	if err := writeManifest(*manifestPath, manifest); err != nil {
		log.Fatalf("write manifest: %v", err)
	}
}

func splitTrim(csv string) []string {
	parts := strings.Split(csv, ",")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

func fingerprint(srcDir, outDir, logical string) (string, error) {
	src := filepath.Join(srcDir, logical)
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])[:10]

	// "app.min.js" → base="app.min", ext=".js" → hashed="app.min-<hash>.js"
	ext := filepath.Ext(logical)
	base := strings.TrimSuffix(logical, ext)
	hashed := fmt.Sprintf("%s-%s%s", base, hash, ext)

	if err := cleanPriors(outDir, base, ext); err != nil {
		return "", fmt.Errorf("clean priors: %w", err)
	}

	dst := filepath.Join(outDir, hashed)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", fmt.Errorf("write copy: %w", err)
	}
	if err := gzipFile(dst, dst+".gz"); err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}

	return hashed, nil
}

// cleanPriors removes any previously-emitted "<base>-<10hex><ext>" file (and
// its .gz sibling) in dir so old fingerprinted copies do not accumulate
// across builds.
func cleanPriors(dir, base, ext string) error {
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(base) + `-[0-9a-f]{10}` + regexp.QuoteMeta(ext) + `(\.gz)?$`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !pattern.MatchString(e.Name()) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	gw, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := io.Copy(gw, in); err != nil {
		gw.Close()
		return err
	}
	return gw.Close()
}

func writeManifest(path string, m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
