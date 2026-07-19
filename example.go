//go:build ignore
// +build ignore

// Example: fetch favicons in multiple formats, sizes, and format preferences.
//
//	go run example.go

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kodehat/favifetch"
)

func main() {
	ctx := context.Background()
	domain := "gmail.com"

	// Temporary output directory (relative to this source file).
	_, thisFile, _, _ := runtime.Caller(0)
	tmpDir := filepath.Join(filepath.Dir(thisFile), "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create tmp dir: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Output directory: %s\n\n", tmpDir)

	// 1. Basic fetch — keep the original format and size.
	fmt.Println("=== 1. Basic fetch (original size & format) ===")
	basic, err := favifetch.Fetch(ctx, domain)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(basic, tmpDir, fmt.Sprintf("basic.%s", basic.Format))
	printResult(basic)

	// 2. Resize to 128×128, keep original format.
	fmt.Println("=== 2. Resize to 128×128 (original format) ===")
	sized, err := favifetch.Fetch(ctx, domain,
		favifetch.WithSize(128),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(sized, tmpDir, fmt.Sprintf("128x128.%s", sized.Format))
	printResult(sized)

	// 3. Resize to 256×256 and convert to PNG.
	fmt.Println("=== 3. Resize to 256×256 + convert to PNG ===")
	png, err := favifetch.Fetch(ctx, domain,
		favifetch.WithSize(256),
		favifetch.WithFormat(favifetch.TargetPNG),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(png, tmpDir, "256x256.png")
	printResult(png)

	// 4. Resize to 64×64 and convert to WebP.
	fmt.Println("=== 4. Resize to 64×64 + convert to WebP ===")
	webp, err := favifetch.Fetch(ctx, domain,
		favifetch.WithSize(64),
		favifetch.WithFormat(favifetch.TargetWebP),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(webp, tmpDir, "64x64.webp")
	printResult(webp)

	// 5. Convert to JPEG (no resize).
	fmt.Println("=== 5. Convert to JPG (original size) ===")
	jpg, err := favifetch.Fetch(ctx, domain,
		favifetch.WithFormat(favifetch.TargetJPEG),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(jpg, tmpDir, "original.jpg")
	printResult(jpg)

	// 6. Discovery — list all sources found.
	fmt.Println("=== 6. Discovery (no download) ===")
	sources, err := favifetch.Discover(ctx, domain,
		favifetch.WithTimeout(10*time.Second),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Discovery error: %v\n", err)
		os.Exit(1)
	}
	for _, s := range sources {
		fmt.Printf("  [%s] score=%d size=%d %s\n", s.Source, s.Score, s.Size, s.URL)
	}

	// 7. Format preference — prefer PNG, fall back to SVG.
	fmt.Println("\n=== 7. Format preference: PNG first, then SVG ===")
	pref, err := favifetch.Fetch(ctx, domain,
		favifetch.WithPreferredFormats(favifetch.FormatPNG, favifetch.FormatSVG),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(pref, tmpDir, fmt.Sprintf("prefer-png.%s", pref.Format))
	printResult(pref)

	// 8. Discovery with format preferences — scores reflect the preference order.
	fmt.Println("=== 8. Discovery with format preferences ===")
	sourcesPref, err := favifetch.Discover(ctx, domain,
		favifetch.WithTimeout(10*time.Second),
		favifetch.WithPreferredFormats(favifetch.FormatPNG, favifetch.FormatICO, favifetch.FormatSVG),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Discovery error: %v\n", err)
		os.Exit(1)
	}
	for _, s := range sourcesPref {
		fmt.Printf("  [%s] score=%d size=%d f=%s %s\n", s.Source, s.Score, s.Size, s.Format, s.URL)
	}

	// 9. Browser mode — use the regular favicon Chromium would select for a tab.
	// Browser mode keeps the original bytes, so it cannot be combined with
	// WithSize or WithFormat.
	fmt.Println("=== 9. Browser mode: Chromium-style tab favicon ===")
	browser, err := favifetch.Fetch(ctx, domain,
		favifetch.WithMode(favifetch.ModeBrowser),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	save(browser, tmpDir, fmt.Sprintf("browser.%s", browser.Format))
	printResult(browser)

	fmt.Println("\nDone! Check", tmpDir)
}

func printResult(r *favifetch.FaviconResult) {
	fmt.Printf("  Format:    %s\n", r.Format)
	fmt.Printf("  Dimensions:%dx%d\n", r.Width, r.Height)
	fmt.Printf("  Size:      %d bytes\n", r.Size)
	fmt.Printf("  Source:    %s\n", r.Source)
	fmt.Printf("  URL:       %s\n\n", r.SourceURL)
}

func save(r *favifetch.FaviconResult, dir, filename string) {
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, r.Data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "  write error: %v\n", err)
		return
	}
	fmt.Printf("  Saved: %s\n", path)
}
