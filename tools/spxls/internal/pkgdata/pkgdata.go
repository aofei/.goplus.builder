package pkgdata

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"fmt"
	"io"
)

//go:generate go run ./gen.go

//go:embed pkgdata.zip
var pkgdata []byte

// OpenExport opens a package export file.
func OpenExport(pkgPath string) (io.ReadCloser, error) {
	zr, err := zip.NewReader(bytes.NewReader(pkgdata), int64(len(pkgdata)))
	if err != nil {
		return nil, fmt.Errorf("create zip reader: %w", err)
	}
	pkgPathWithExt := pkgPath + ".export"
	for _, f := range zr.File {
		if f.Name == pkgPathWithExt {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("package %q not found", pkgPath)
}
