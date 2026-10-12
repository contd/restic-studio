package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	source := []byte("# Example heading\n\n[Manual](./README.md#preview) [Development](DEVELOPMENT.md)\n\n" +
		"| Name | Value |\n| --- | --- |\n| One | Two |\n\n" +
		"<details><summary>More</summary>Details</details>\n\n" +
		"```mermaid\nflowchart LR\nA --> B\n```\n")
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`id="example-heading"`, `href="index.html#preview"`, `href="development.html"`,
		"<table>", "<details>", `class="language-mermaid"`,
	} {
		if !strings.Contains(string(body), expected) {
			t.Errorf("missing rendered element %q", expected)
		}
	}
}

func TestBuildSite(t *testing.T) {
	root := t.TempDir()
	for name, contents := range map[string]string{
		"README.md":                       "# Manual\n\n![Preview](docs/images/demo.gif)",
		"DEVELOPMENT.md":                  "# Development\n",
		"docs/images/demo.gif":            "demo image",
		"docs/recordings/demo.cast":       "demo recording",
		"testdata/autorestic.example.yml": "version: 2\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(root, "_site")
	for attempt := 0; attempt < 2; attempt++ {
		if err := buildSite(root, output); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"index.html", "development.html", ".nojekyll",
		"docs/images/demo.gif", "docs/recordings/demo.cast", "testdata/autorestic.example.yml"} {
		data, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, ".html") && !bytes.Contains(data, []byte(`href="development.html"`)) {
			t.Errorf("%s has no development navigation", name)
		}
		if strings.HasSuffix(name, ".gif") && string(data) != "demo image" {
			t.Error("image was not copied unchanged")
		}
	}
}

func TestBuildSiteRejectsSourceOutput(t *testing.T) {
	root := t.TempDir()
	for _, output := range []string{root, filepath.Dir(root), filepath.Join(root, "docs"),
		filepath.Join(root, "docs", "images", "nested"), filepath.Join(root, "cmd", "docs")} {
		if err := buildSite(root, output); err == nil {
			t.Errorf("accepted source-overlapping output %s", output)
		}
	}
}

func TestBuildSiteReportsMissingInput(t *testing.T) {
	root := t.TempDir()
	if err := buildSite(root, filepath.Join(root, "_site")); err == nil {
		t.Fatal("missing README was not reported")
	}
}
