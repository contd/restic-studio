package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
)

//go:embed page.html
var pageTemplate string

func main() {
	output := flag.String("output", "_site", "documentation site output directory")
	flag.Parse()
	if err := buildSite(".", *output); err != nil {
		fmt.Fprintln(os.Stderr, "build documentation site:", err)
		os.Exit(1)
	}
	fmt.Println("Documentation site generated in", *output)
}

func buildSite(root, output string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	// Refuse output paths that could overwrite source files or capture assets.
	for _, source := range []string{root, filepath.Join(root, "docs"), filepath.Join(root, "cmd"), filepath.Join(root, "testdata")} {
		rel, err := filepath.Rel(output, source)
		if err != nil {
			return err
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return fmt.Errorf("output directory %q contains project sources", output)
		}
		if source != root {
			rel, err = filepath.Rel(source, output)
			if err != nil {
				return err
			}
			if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("output directory %q is inside project sources", output)
			}
		}
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	layout, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return err
	}
	for _, page := range []struct{ source, destination, title string }{
		{"README.md", "index.html", "Restic Studio"},
		{"DEVELOPMENT.md", "development.html", "Developing Restic Studio"},
	} {
		source, err := os.ReadFile(filepath.Join(root, page.source))
		if err != nil {
			return err
		}
		body, err := renderMarkdown(source)
		if err != nil {
			return fmt.Errorf("render %s: %w", page.source, err)
		}
		var rendered bytes.Buffer
		if err := layout.Execute(&rendered, struct {
			Title string
			Body  template.HTML
		}{page.title, template.HTML(body)}); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(output, page.destination), rendered.Bytes(), 0644); err != nil {
			return err
		}
	}
	for _, directory := range []string{"docs/images", "docs/recordings", "testdata"} {
		if err := copyAssets(root, output, directory); err != nil {
			return fmt.Errorf("copy %s: %w", directory, err)
		}
	}
	return os.WriteFile(filepath.Join(output, ".nojekyll"), nil, 0644)
}

func renderMarkdown(source []byte) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		// The inputs are trusted project documents; preserve their details elements.
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	document := md.Parser().Parse(text.NewReader(source))
	if err := ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, ok := node.(*ast.Link); ok && entering {
			destination := strings.TrimPrefix(string(link.Destination), "./")
			path, fragment, hasFragment := strings.Cut(destination, "#")
			switch path {
			case "README.md":
				path = "index.html"
			case "DEVELOPMENT.md":
				path = "development.html"
			default:
				return ast.WalkContinue, nil
			}
			if hasFragment {
				path += "#" + fragment
			}
			link.Destination = []byte(path)
		}
		return ast.WalkContinue, nil
	}); err != nil {
		return nil, err
	}
	var body bytes.Buffer
	err := md.Renderer().Render(&body, source, document)
	return body.Bytes(), err
}

func copyAssets(root, output, directory string) error {
	return filepath.WalkDir(filepath.Join(root, filepath.FromSlash(directory)), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("asset symlinks are not supported: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(output, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0644)
	})
}
