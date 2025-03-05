package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"gopkg.in/yaml.v3"
)

// Configuration holds the command line arguments
type Configuration struct {
	InputDir     string
	OutputDir    string
	TemplatesDir string
	Serve        bool
	Host         string
	Port         int
}

// FrontMatter represents the YAML front matter in markdown files
type FrontMatter struct {
	Title       string                 `yaml:"title,omitempty"`
	Template    string                 `yaml:"template,omitempty"`
	Description string                 `yaml:"description,omitempty"`
	Date        string                 `yaml:"date,omitempty"`
	Tags        []string               `yaml:"tags,omitempty"`
	Custom      map[string]interface{} `yaml:",inline"`
}

// Colors for console output
var (
	blue    = color.New(color.FgBlue).SprintFunc()
	green   = color.New(color.FgGreen).SprintFunc()
	yellow  = color.New(color.FgYellow).SprintFunc()
	red     = color.New(color.FgRed).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
	cyan    = color.New(color.FgCyan).SprintFunc()
)

func main() {
	// Parse command line arguments
	config := parseFlags()

	// Either serve with live reload or perform a one-time build
	if config.Serve {
		serve(config)
	} else {
		build(config)
	}
}

// parseFlags parses command line arguments
func parseFlags() Configuration {
	inputDir := flag.String("i", "input", "Input directory containing source files")
	outputDir := flag.String("o", "output", "Output directory for generated site")
	templatesDir := flag.String("t", "templates", "Directory containing templates")
	serve := flag.Bool("s", false, "Start development server after build")
	host := flag.String("host", "0.0.0.0", "Host address to bind development server")
	port := flag.Int("port", 8000, "Port for development server")

	flag.Parse()

	return Configuration{
		InputDir:     *inputDir,
		OutputDir:    *outputDir,
		TemplatesDir: *templatesDir,
		Serve:        *serve,
		Host:         *host,
		Port:         *port,
	}
}

// build processes the input directory and generates the static site
func build(config Configuration) {
	fmt.Println(blue("Build Configuration:"))
	fmt.Printf("%s %s\n", yellow("Input directory:"), config.InputDir)
	fmt.Printf("%s %s\n", yellow("Output directory:"), config.OutputDir)
	fmt.Printf("%s %s\n", yellow("Templates directory:"), config.TemplatesDir)

	// Check if directories exist
	if _, err := os.Stat(config.InputDir); os.IsNotExist(err) {
		fmt.Printf("%s Input directory does not exist: %s\n", red("Error:"), config.InputDir)
		return
	}
	if _, err := os.Stat(config.TemplatesDir); os.IsNotExist(err) {
		fmt.Printf("%s Templates directory does not exist: %s\n", red("Error:"), config.TemplatesDir)
		return
	}

	// List all files in input directory
	fmt.Println(blue("\nFiles found in input directory:"))
	err := filepath.Walk(config.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(config.InputDir, path)
			fmt.Printf("%s %s\n", green("Found:"), relPath)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("%s Error walking input directory: %s\n", red("Error:"), err)
		return
	}

	// Start with a clean output directory
	os.RemoveAll(config.OutputDir)
	os.MkdirAll(config.OutputDir, 0o755)

	// Track processing metrics
	start := time.Now()
	count := 0

	// Process all files in input directory
	err = filepath.Walk(config.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(config.InputDir, path)
		outputPath := filepath.Join(config.OutputDir, relPath)

		if info.IsDir() {
			// Create directory structure in output
			os.MkdirAll(outputPath, 0o755)
		} else if isMarkdownFile(path) {
			// Convert markdown files to HTML
			renderMarkdown(path, outputPath, config.TemplatesDir)
			count++
		} else if !strings.HasPrefix(filepath.Base(path), ".") { // Skip hidden files
			// Copy all other files as-is
			os.MkdirAll(filepath.Dir(outputPath), 0o755)
			copyFile(path, outputPath)
			count++
		}

		return nil
	})
	if err != nil {
		fmt.Printf("%s Error processing files: %s\n", red("Error:"), err)
		return
	}

	// Handle robots.txt after processing other files
	handleRobotsTxt(config.InputDir, config.OutputDir)

	// Log build completion statistics
	elapsed := time.Since(start).Seconds()
	fmt.Printf("%s Processed %d files in %.2f seconds.\n", green("✓"), count, elapsed)
}

// isMarkdownFile checks if a file is a markdown file
func isMarkdownFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".md" || ext == ".markdown" || ext == ".mkd" || ext == ".mdown"
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

// parseFrontMatter parses the YAML front matter from a markdown file
func parseFrontMatter(content string) (FrontMatter, string) {
	frontMatter := FrontMatter{}

	// Check if the file has front matter (starts with ---)
	if !strings.HasPrefix(content, "---\n") {
		return frontMatter, content
	}

	// Find the end of the front matter
	endIndex := strings.Index(content[4:], "---\n")
	if endIndex == -1 {
		return frontMatter, content
	}

	// Extract front matter and content
	frontMatterContent := content[4 : endIndex+4]
	markdownContent := content[endIndex+8:]

	// Parse YAML front matter
	err := yaml.Unmarshal([]byte(frontMatterContent), &frontMatter)
	if err != nil {
		fmt.Printf("%s Error parsing front matter: %s\n", red("Error:"), err)
	}

	return frontMatter, markdownContent
}

// renderMarkdown converts a markdown file to HTML using template
func renderMarkdown(inputPath, outputPath, templatesDir string) {
	// Read markdown file
	content, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Printf("%s Error reading markdown file: %s\n", red("Error:"), err)
		return
	}

	// Parse front matter and content
	frontMatter, markdownContent := parseFrontMatter(string(content))

	// Convert markdown to HTML using goldmark
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Typographer),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)
	var buf bytes.Buffer
	if err := md.Convert([]byte(markdownContent), &buf); err != nil {
		fmt.Printf("%s Error converting markdown: %s\n", red("Error:"), err)
		return
	}
	htmlContent := buf.String()

	fmt.Printf("%s %s\n", blue("Processing"), inputPath)
	fmt.Printf("%s %s\n", green("Template:"), frontMatter.Template)
	fmt.Printf("%s %d\n", yellow("Content length:"), len(markdownContent))

	// Get list of available templates for logging
	var templates []string
	filepath.Walk(templatesDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && (strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".tmpl")) {
			relPath, _ := filepath.Rel(templatesDir, path)
			templates = append(templates, relPath)
		}
		return nil
	})
	fmt.Printf("%s %s\n", magenta("Available templates:"), strings.Join(templates, ", "))

	// Prepare data for template
	templateData := map[string]interface{}{
		"Title":       frontMatter.Title,
		"Description": frontMatter.Description,
		"Date":        frontMatter.Date,
		"Content":     template.HTML(htmlContent), // Mark as safe HTML
	}

	// Add any custom fields from front matter
	for k, v := range frontMatter.Custom {
		templateData[k] = v
	}

	// Load template
	templateFile := frontMatter.Template
	if templateFile == "" {
		templateFile = "default.html"
	}

	tmplPath := filepath.Join(templatesDir, templateFile)
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		fmt.Printf("%s Error loading template %s: %s\n", red("Error:"), templateFile, err)
		// Fall back to direct HTML output
		htmlOutput := fmt.Sprintf("<html><body>%s</body></html>", htmlContent)
		writeHTMLFile(outputPath, inputPath, []byte(htmlOutput))
		return
	}

	// Render template
	var output bytes.Buffer
	if err := tmpl.Execute(&output, templateData); err != nil {
		fmt.Printf("%s Error executing template: %s\n", red("Error:"), err)
		// Fall back to direct HTML output
		htmlOutput := fmt.Sprintf("<html><body>%s</body></html>", htmlContent)
		writeHTMLFile(outputPath, inputPath, []byte(htmlOutput))
		return
	}

	writeHTMLFile(outputPath, inputPath, output.Bytes())
}

// writeHTMLFile determines the output path and writes HTML content
func writeHTMLFile(outputPath, inputPath string, content []byte) {
	// Determine output path
	outputHTMLPath := outputPath
	if filepath.Base(inputPath) == "index.md" {
		// For index.md files, keep the same directory structure
		outputHTMLPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".html"
	} else {
		// For other files, create a directory and place index.html inside
		baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
		outputHTMLPath = filepath.Join(filepath.Dir(outputPath), baseName, "index.html")
	}

	// Ensure output directory exists
	os.MkdirAll(filepath.Dir(outputHTMLPath), 0o755)

	fmt.Printf("Writing HTML to: %s\n", outputHTMLPath)

	// Write HTML to file
	err := os.WriteFile(outputHTMLPath, content, 0o644)
	if err != nil {
		fmt.Printf("%s Error writing HTML file: %s\n", red("Error:"), err)
	}
}

// handleRobotsTxt handles the robots.txt file for the site
func handleRobotsTxt(inputDir, outputDir string) {
	inputRobots := filepath.Join(inputDir, "robots.txt")
	outputRobots := filepath.Join(outputDir, "robots.txt")

	if _, err := os.Stat(inputRobots); err == nil {
		// Copy existing robots.txt
		copyFile(inputRobots, outputRobots)
		fmt.Println("Copied existing robots.txt")
	} else {
		// Generate default robots.txt
		defaultRobots := `User-agent: *
Allow: /
Sitemap: sitemap.xml`

		err := os.WriteFile(outputRobots, []byte(defaultRobots), 0o644)
		if err != nil {
			fmt.Printf("%s Error writing robots.txt: %s\n", red("Error:"), err)
		} else {
			fmt.Println("Generated default robots.txt")
		}
	}
}

// serve starts a development server and watches for file changes
func serve(config Configuration) {
	// Build site initially
	build(config)

	// Setup HTTP server for serving files
	http.Handle("/", http.FileServer(http.Dir(config.OutputDir)))

	serverAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	fmt.Printf("\n%s http://%s\n", green("Starting server at"), serverAddr)
	fmt.Printf("%s\n", yellow("Press Ctrl+C to quit"))
	fmt.Printf("%s\n", blue("Watching for changes in:"))
	fmt.Printf("%s %s\n", blue("- Input:"), config.InputDir)
	fmt.Printf("%s %s\n", blue("- Templates:"), config.TemplatesDir)

	// Start HTTP server in a goroutine
	go func() {
		if err := http.ListenAndServe(serverAddr, logRequest(http.DefaultServeMux)); err != nil {
			fmt.Printf("%s Server error: %s\n", red("Error:"), err)
			os.Exit(1)
		}
	}()

	// Create channel for clean termination
	done := make(chan bool)

	// Store file modification times
	lastModified := make(map[string]time.Time)
	lastRebuild := time.Now()

	// Initialize the lastModified map with current file information
	initializeFileMap := func(dir string) {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				lastModified[path] = info.ModTime()
			}
			return nil
		})
	}

	// Initialize with current file state
	initializeFileMap(config.InputDir)
	initializeFileMap(config.TemplatesDir)

	// Start file change detection goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				filesChanged := false
				changedSource := ""
				changedFile := ""

				// Check for changes in input directory
				err := filepath.Walk(config.InputDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					// Skip hidden files and directories
					if strings.HasPrefix(filepath.Base(path), ".") {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}

					// Skip temporary files
					if strings.HasSuffix(path, ".tmp") {
						return nil
					}

					// Check if file is new or modified
					if !info.IsDir() {
						modTime := info.ModTime()
						lastMod, exists := lastModified[path]

						if !exists || modTime.After(lastMod) {
							filesChanged = true
							changedSource = "input files"
							changedFile = path
							lastModified[path] = modTime
						}
					}
					return nil
				})
				if err != nil {
					fmt.Printf("%s Error checking for file changes: %s\n", red("Error:"), err)
				}

				// Check for changes in templates directory
				if !filesChanged {
					err := filepath.Walk(config.TemplatesDir, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}

						// Skip hidden files and directories
						if strings.HasPrefix(filepath.Base(path), ".") {
							if info.IsDir() {
								return filepath.SkipDir
							}
							return nil
						}

						// Check if file is new or modified
						if !info.IsDir() {
							modTime := info.ModTime()
							lastMod, exists := lastModified[path]

							if !exists || modTime.After(lastMod) {
								filesChanged = true
								changedSource = "template files"
								changedFile = path
								lastModified[path] = modTime
							}
						}
						return nil
					})
					if err != nil {
						fmt.Printf("%s Error checking for file changes: %s\n", red("Error:"), err)
					}
				}

				// If files changed and enough time has passed since the last rebuild, trigger a rebuild
				if filesChanged && time.Since(lastRebuild).Seconds() >= 1 {
					fmt.Printf("\n%s %s\n", yellow("Changes detected in "+changedSource+":"), changedFile)
					fmt.Printf("%s\n", cyan("Rebuilding entire site..."))

					build(config)
					fmt.Printf("%s\n", green("Rebuild complete!"))

					lastRebuild = time.Now()

					// Update the file map after rebuild
					initializeFileMap(config.InputDir)
					initializeFileMap(config.TemplatesDir)
				}
			case <-done:
				return
			}
		}
	}()

	<-done // Block forever
}

// logRequest wraps an http.Handler with request logging
func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the original handler
		handler.ServeHTTP(w, r)

		// Log the request with colors
		fmt.Printf("%s %s %s %s %s\n",
			cyan(r.RemoteAddr),
			time.Now().Format("02/Jan/2006:15:04:05 -0700"),
			magenta(r.URL.Path),
			r.Method,
			yellow(time.Since(start).String()),
		)
	})
}
