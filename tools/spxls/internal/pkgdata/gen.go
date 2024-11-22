//go:build ignore

package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// pkgs is the list of packages to generate the exported symbols for.
var pkgs = []string{
	"fmt",
	"math/...",
	// TODO: Add more packages here.

	"github.com/goplus/spx/...",
}

// generate generates the pkgdata.zip file containing the exported symbols of the given packages.
func generate() error {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, pkg := range pkgs {
		cmd := exec.Command("go", "list", "-f", "{{.ImportPath}}:{{.Export}}", "-export", pkg)
		cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
		output, err := cmd.Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				err = fmt.Errorf("%s: %s", ee, ee.Stderr)
			}
			return fmt.Errorf("failed to go list: %w", err)
		}

		for _, line := range strings.Split(string(output), "\n") {
			if line == "" {
				continue
			}

			pkgPath, exportFile, _ := strings.Cut(line, ":")
			data, err := os.ReadFile(exportFile)
			if err != nil {
				return err
			}

			f, err := zw.Create(pkgPath + ".export")
			if err != nil {
				return err
			}
			if _, err = f.Write(data); err != nil {
				return err
			}
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile("pkgdata.zip", buf.Bytes(), 0644)
}

func main() {
	if err := generate(); err != nil {
		panic(err)
	}
}
