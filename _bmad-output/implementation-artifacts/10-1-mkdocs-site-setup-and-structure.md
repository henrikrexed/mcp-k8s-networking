# Story 10.1: MkDocs Site Setup and Structure

Status: done

## Story

As a documentation consumer,
I want a Material for MkDocs documentation site with structured navigation,
so that I can find information about the MCP server's tools, architecture, and deployment.

## Acceptance Criteria

1. `mkdocs.yml` is configured with Material theme, light/dark toggle, navigation tabs, code copy, and search highlighting
2. Navigation structure covers: Home, Getting Started (Quick Start, Configuration), Tools Reference (10 pages), Architecture, Contributing
3. Markdown extensions include admonition, superfences, tabbed, highlight, inlinehilite, and TOC with permalinks
4. A GitHub Actions workflow at `.github/workflows/docs.yml` deploys to GitHub Pages on push to main when docs/ or mkdocs.yml change
5. The workflow uses actions/setup-python, installs mkdocs-material, builds with mkdocs build, and deploys via actions/deploy-pages

## Tasks / Subtasks

- [x] Create mkdocs.yml with site_name, site_description, site_url, repo_url, repo_name
- [x] Configure Material theme with blue primary/indigo accent, light/dark palette toggle
- [x] Enable theme features: navigation.tabs, navigation.sections, navigation.expand, content.code.copy, search.highlight
- [x] Define nav structure with Home, Getting Started, Tools Reference (10 sub-pages), Architecture, Contributing
- [x] Configure markdown extensions: admonition, pymdownx.details, pymdownx.superfences, pymdownx.tabbed, pymdownx.highlight, pymdownx.inlinehilite, toc with permalink
- [x] Create .github/workflows/docs.yml with push trigger on main branch (paths: docs/**, mkdocs.yml)
- [x] Configure workflow permissions (contents: read, pages: write, id-token: write) and concurrency group
- [x] Implement build job: checkout, setup-python, pip install mkdocs-material, mkdocs build, upload-pages-artifact
- [x] Implement deploy job: deploy-pages action with environment github-pages

## Dev Notes

### Theme Configuration

The Material theme is configured with dual palette (default/slate) for light/dark mode toggle. Navigation features enable tabbed top-level sections, expandable navigation tree, and code block copy buttons.

### GitHub Actions Workflow

The docs workflow is triggered only when documentation files change (docs/** or mkdocs.yml). It uses the modern GitHub Pages deployment pattern with upload-pages-artifact and deploy-pages actions, which provides better control and doesn't require a gh-pages branch.

### Navigation Structure

```
Home
Getting Started/
  Quick Start
  Configuration
Tools Reference/
  Overview
  Core Kubernetes
  Gateway API
  Istio
  kgateway
  Tier 2 Providers
  Log Collection
  Active Probing
  Design Guidance
  Agent Skills
Architecture
Contributing
```

### Concurrency

The workflow uses `concurrency: group: "pages"` with `cancel-in-progress: true` to prevent overlapping deployments.

## File List

| File | Action |
|---|---|
| `mkdocs.yml` | Created |
| `.github/workflows/docs.yml` | Created |
