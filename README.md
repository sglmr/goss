# GoSS - Go Static Site Generator

A simple and fast static site generator written in Go that converts Markdown files to HTML using templates.

## Features

- Markdown to HTML conversion with YAML frontmatter support
- Template-based rendering using Go's html/template
- Development server with live reload
- Clean URL structure (e.g., `/blog/post/` instead of `/blog/post.html`)
- Automatic robots.txt generation
- Colorized console output

## Installation

### Prerequisites

- Go 1.16 or higher

### Dependencies

```bash
go get github.com/fatih/color
go get github.com/yuin/goldmark
go get github.com/yuin/goldmark/extension
go get gopkg.in/yaml.v3
```

### Building

```bash
git clone https://github.com/yourusername/goss
cd goss
go build
```

## Usage

### Basic Commands

```bash
# One-time build
./goss -i input -o output -t templates

# Start development server with live reload
./goss -i input -o output -t templates -s
```

### Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-i` | input | Input directory containing source files |
| `-o` | output | Output directory for generated site |
| `-t` | templates | Directory containing templates |
| `-s` | false | Start development server after build |
| `--host` | 0.0.0.0 | Host address for development server |
| `--port` | 8000 | Port for development server |

## Project Structure

### Directories

```
your-project/
├── input/           # Source content (Markdown files)
├── templates/       # HTML templates
└── output/          # Generated site (created by the tool)
```

### Example Markdown File

```markdown
---
title: Hello World
description: My first post
template: default.html
date: 2025-03-05
---

# Hello World

This is my first post.
```

### Example Template

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }}</title>
    <meta name="description" content="{{ .Description }}">
</head>
<body>
    <header>
        <h1>{{ .Title }}</h1>
    </header>
    
    <main>
        {{ .Content }}
    </main>
    
    <footer>
        {{ if .Date }}Published: {{ .Date }}{{ end }}
    </footer>
</body>
</html>
```

## Front Matter

The following fields are supported in the YAML front matter:

| Field | Description |
|-------|-------------|
| `title` | Page title |
| `description` | Page description |
| `template` | Template file to use (defaults to default.html) |
| `date` | Publication date |
| `tags` | Array of tags |

Custom fields can also be added and will be available in templates.

## Development Server

The development server watches both the input and templates directories for changes and automatically rebuilds the site when files are modified.

Access your site at http://localhost:8000 (or the specified host/port).

## License

MIT License
