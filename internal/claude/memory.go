package claude

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"
	"gopkg.in/yaml.v3"
)

type Memory struct {
	Project     string
	Name        string
	Slug        string
	File        string
	Type        string
	Description string
}

type MemoryPage struct {
	Title       string
	Type        string
	Description string
	Body        string
}

type memoryFile struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		Type string `yaml:"type"`
	} `yaml:"metadata"`
}

type MemoryIssue struct {
	Project string
	Kind    string
	Name    string
}

func ListMemories(projectPath string) ([]Memory, error) {
	projects, err := projectPaths(projectPath)
	if err != nil {
		return nil, err
	}
	var out []Memory
	for _, project := range projects {
		entries, _, err := readMemoryDir(project)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func ListMemoryIssues() ([]MemoryIssue, error) {
	projects, err := projectPaths("")
	if err != nil {
		return nil, err
	}
	var out []MemoryIssue
	for _, project := range projects {
		entries, files, err := readMemoryDir(project)
		if err != nil {
			return nil, err
		}
		indexed := map[string]bool{}
		for _, entry := range entries {
			if !files[entry.File] {
				out = append(out, MemoryIssue{
					Project: project,
					Kind:    "indexed, missing file",
					Name:    entry.Name,
				})
			}
			indexed[entry.File] = true
		}
		var orphans []string
		for name := range files {
			if !indexed[name] {
				orphans = append(orphans, name)
			}
		}
		sort.Strings(orphans)
		for _, name := range orphans {
			label := strings.TrimSuffix(name, filepath.Ext(name))
			if meta, ok := readMemoryFile(filepath.Join(MemoryDir(project), name)); ok && meta.Name != "" {
				label = meta.Name
			}
			out = append(out, MemoryIssue{
				Project: project,
				Kind:    "file, not indexed",
				Name:    label,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project != out[j].Project {
			return out[i].Project < out[j].Project
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func ReadMemory(projectPath, name string) (MemoryPage, error) {
	project, err := ResolveProject(projectPath)
	if err != nil {
		return MemoryPage{}, err
	}
	entries, files, err := readMemoryDir(project)
	if err != nil {
		return MemoryPage{}, err
	}
	var chosen *Memory
	var file string
	for i := range entries {
		entry := &entries[i]
		if entry.Name == name || entry.Slug == name || entry.File == name || strings.TrimSuffix(entry.File, filepath.Ext(entry.File)) == name {
			chosen = entry
			file = entry.File
			break
		}
	}
	if file == "" {
		for filename := range files {
			if filename == name || strings.TrimSuffix(filename, filepath.Ext(filename)) == name {
				file = filename
				break
			}
		}
	}
	if file == "" {
		return MemoryPage{}, fmt.Errorf("no memory %q in %s", name, project)
	}
	data, err := os.ReadFile(filepath.Join(MemoryDir(project), file))
	if err != nil {
		return MemoryPage{}, err
	}
	front, body := splitFrontmatter(string(data))
	meta, _ := parseMemoryFile(front)
	title := ""
	description := meta.Description
	if chosen != nil {
		title = chosen.Name
		if description == "" {
			description = chosen.Description
		}
	}
	if title == "" {
		title = meta.Name
	}
	return MemoryPage{
		Title:       title,
		Type:        meta.Metadata.Type,
		Description: description,
		Body:        strings.TrimSpace(body),
	}, nil
}

func CurrentProject() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("current directory is not a folder: %s", cwd)
	}
	projects, err := projectPaths("")
	if err != nil {
		return "", err
	}
	for _, project := range projects {
		abs, err := filepath.Abs(project)
		if err != nil {
			continue
		}
		if cwd == abs {
			return abs, nil
		}
	}
	return "", fmt.Errorf("current directory is not a project folder: %s", cwd)
}

func ResolveProject(arg string) (string, error) {
	projects, err := projectPaths(arg)
	if err != nil {
		return "", err
	}
	return projects[0], nil
}

func projectPaths(filter string) ([]string, error) {
	projects, err := ListProjects()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(projects))
	for _, project := range projects {
		paths = append(paths, project.Path)
	}
	sort.Strings(paths)
	if filter == "" {
		return paths, nil
	}
	resolved, err := resolveIn(paths, filter)
	if err != nil {
		return nil, err
	}
	return []string{resolved}, nil
}

func resolveIn(projects []string, arg string) (string, error) {
	var matches []string
	for _, project := range projects {
		if project == arg || strings.HasSuffix(project, "/"+arg) {
			matches = append(matches, project)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no project %q", arg)
	default:
		return "", fmt.Errorf("project %q matches %s", arg, strings.Join(matches, ", "))
	}
}

func readMemoryDir(project string) ([]Memory, map[string]bool, error) {
	dir := MemoryDir(project)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, map[string]bool{}, nil
		}
		return nil, nil, err
	}
	files := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "MEMORY.md" {
			continue
		}
		files[entry.Name()] = true
	}
	index, err := readMemoryIndex(project, filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		return nil, nil, err
	}
	for i := range index {
		applyFrontmatter(&index[i], dir)
	}
	return index, files, nil
}

func readMemoryIndex(project, path string) ([]Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	doc := parser.New().Parse(data)
	var out []Memory
	err = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		item, ok := n.(*ast.ListItem)
		if !ok {
			return ast.WalkContinue, nil
		}
		if mem, ok := memoryFromItem(item, data, project); ok {
			out = append(out, mem)
		}
		return ast.WalkSkipChildren, nil
	})
	return out, err
}

func memoryFromItem(item ast.Node, source []byte, project string) (Memory, bool) {
	link := firstLink(item)
	if link == nil {
		return Memory{}, false
	}
	title := strings.TrimSpace(nodeText(link, source))
	file := filepath.Base(link.Destination.Value(source))
	if title == "" || file == "" || file == "." {
		return Memory{}, false
	}
	return Memory{
		Project:     project,
		Name:        title,
		File:        file,
		Description: descriptionAfterLink(item, link, source),
	}, true
}

func firstLink(n ast.Node) *ast.Link {
	var link *ast.Link
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || link != nil {
			return ast.WalkContinue, nil
		}
		if found, ok := n.(*ast.Link); ok {
			link = found
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return link
}

func descriptionAfterLink(item ast.Node, link *ast.Link, source []byte) string {
	var b strings.Builder
	seen := false
	_ = ast.Walk(item, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if n == link {
			if entering {
				seen = true
			}
			return ast.WalkSkipChildren, nil
		}
		if !entering || !seen {
			return ast.WalkContinue, nil
		}
		if t, ok := n.(*ast.Text); ok {
			b.WriteString(t.Value.Value(source))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})
	desc := strings.TrimSpace(b.String())
	desc = strings.TrimPrefix(desc, "—")
	desc = strings.TrimPrefix(desc, "-")
	return strings.TrimSpace(desc)
}

func nodeText(n ast.Node, source []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if t, ok := n.(*ast.Text); ok {
			b.WriteString(t.Value.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func applyFrontmatter(m *Memory, dir string) {
	meta, ok := readMemoryFile(filepath.Join(dir, m.File))
	if !ok {
		return
	}
	m.Slug = meta.Name
	if meta.Description != "" {
		m.Description = meta.Description
	}
	m.Type = meta.Metadata.Type
}

func readMemoryFile(path string) (memoryFile, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return memoryFile{}, false
	}
	front, _ := splitFrontmatter(string(data))
	meta, ok := parseMemoryFile(front)
	return meta, ok
}

func parseMemoryFile(front string) (memoryFile, bool) {
	if strings.TrimSpace(front) == "" {
		return memoryFile{}, false
	}
	var meta memoryFile
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		return memoryFile{}, false
	}
	return meta, true
}

func splitFrontmatter(raw string) (string, string) {
	s := strings.TrimPrefix(raw, "\uFEFF")
	if !strings.HasPrefix(s, "---") {
		return "", strings.TrimSpace(s)
	}
	rest := strings.TrimPrefix(s, "---")
	rest = strings.TrimPrefix(rest, "\r\n")
	rest = strings.TrimPrefix(rest, "\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", strings.TrimSpace(s)
	}
	front := rest[:end]
	body := rest[end+len("\n---"):]
	body = strings.TrimPrefix(body, "\r\n")
	body = strings.TrimPrefix(body, "\n")
	return front, body
}
