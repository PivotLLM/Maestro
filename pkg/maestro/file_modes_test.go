//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestExtractZipFile_UsesConfiguredModes: extracted entries get the given
// directory and file modes, not the modes recorded in the archive.
func TestExtractZipFile_UsesConfiguredModes(t *testing.T) {
	testutil.NarrowUmask(t)
	const dirMode, fileMode fs.FileMode = 0o770, 0o660

	zipPath := filepath.Join(t.TempDir(), "alice.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	for _, e := range []struct {
		name string
		mode fs.FileMode
	}{
		{"exec.sh", 0o755},
		{"sub/bob.txt", 0o644},
	} {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		hdr.SetMode(e.mode)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(t.TempDir(), "out")
	extracted, skipped, err := extractZipFile(zipPath, destDir, false, dirMode, fileMode, nil)
	if err != nil || extracted != 2 || skipped != 0 {
		t.Fatalf("extractZipFile: extracted=%d skipped=%d err=%v", extracted, skipped, err)
	}
	testutil.AssertPerm(t, destDir, dirMode)
	testutil.AssertPerm(t, filepath.Join(destDir, "sub"), dirMode)
	testutil.AssertPerm(t, filepath.Join(destDir, "exec.sh"), fileMode)
	testutil.AssertPerm(t, filepath.Join(destDir, "sub", "bob.txt"), fileMode)
}
