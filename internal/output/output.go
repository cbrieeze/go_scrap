package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go_scrap/internal/menu"
	"go_scrap/internal/parse"
	"go_scrap/internal/report"
)

type WriteOptions struct {
	OutputDir    string
	MarkdownFile string
	JSONFile     string
}

type ChunkLimits struct {
	MaxBytes  int
	MaxChars  int
	MaxTokens int
}

func (c ChunkLimits) Enabled() bool {
	return c.MaxBytes > 0 || c.MaxChars > 0 || c.MaxTokens > 0
}

type chunkSize struct {
	bytes  int
	chars  int
	tokens int
}

func (c ChunkLimits) exceeds(size chunkSize) bool {
	if c.MaxBytes > 0 && size.bytes > c.MaxBytes {
		return true
	}
	if c.MaxChars > 0 && size.chars > c.MaxChars {
		return true
	}
	if c.MaxTokens > 0 && size.tokens > c.MaxTokens {
		return true
	}
	return false
}

type JSONDoc struct {
	HeadingIDs    []string        `json:"heading_ids"`
	AnchorTargets []string        `json:"anchor_targets"`
	Sections      []parse.Section `json:"sections"`
	Report        report.Report   `json:"report"`
}

func WriteAll(doc *parse.Document, rep report.Report, markdown string, opts WriteOptions) (string, string, error) {
	mdPath, err := WriteMarkdown(opts.OutputDir, opts.MarkdownFile, markdown)
	if err != nil {
		return "", "", err
	}
	jsonPath, err := WriteJSON(doc, rep, opts)
	if err != nil {
		return "", "", err
	}
	return mdPath, jsonPath, nil
}

func WriteJSON(doc *parse.Document, rep report.Report, opts WriteOptions) (string, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = "output"
	}
	if opts.JSONFile == "" {
		opts.JSONFile = "content.json"
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return "", err
	}

	jsonPath := filepath.Join(opts.OutputDir, opts.JSONFile)
	payload := JSONDoc{
		HeadingIDs:    doc.HeadingIDs,
		AnchorTargets: doc.AnchorTargets,
		Sections:      doc.Sections,
		Report:        rep,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(jsonPath, data, 0600); err != nil {
		return "", err
	}

	return jsonPath, nil
}

func WriteMarkdown(outputDir string, filename string, markdown string) (string, error) {
	if outputDir == "" {
		outputDir = "output"
	}
	if filename == "" {
		filename = "content.md"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	mdPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(mdPath, []byte(markdown), 0600); err != nil {
		return "", err
	}
	return mdPath, nil
}

func WriteMarkdownParts(outputDir string, filename string, parts []string, limits ChunkLimits) (string, error) {
	if outputDir == "" {
		outputDir = "output"
	}
	if filename == "" {
		filename = "content.md"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	if !limits.Enabled() {
		return WriteMarkdown(outputDir, filename, strings.Join(parts, ""))
	}

	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	basePath := filepath.Join(outputDir, baseName)
	bundles := bundleParts(parts, limits)
	if len(bundles) <= 1 {
		return WriteMarkdown(outputDir, filename, strings.Join(parts, ""))
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return "", err
	}

	for i, bundle := range bundles {
		partPath := filepath.Join(basePath, fmt.Sprintf("part-%03d.md", i+1))
		if err := os.WriteFile(partPath, []byte(bundle), 0600); err != nil {
			return "", err
		}
	}

	heading := firstHeadingLine(strings.Join(parts, ""))
	index := buildSplitIndex(heading, baseName, len(bundles))
	mdPath := filepath.Join(outputDir, filename)
	if err := os.WriteFile(mdPath, []byte(index), 0600); err != nil {
		return "", err
	}
	return mdPath, nil
}

func WriteMenu(outputDir string, nodes []menu.Node) error {
	if outputDir == "" {
		outputDir = "output"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(outputDir, "menu.json")
	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func WriteSectionFiles(outputDir string, nodes []menu.Node, mdByID map[string]string, maxItems int, limits ChunkLimits) error {
	if outputDir == "" {
		outputDir = "output"
	}
	base := filepath.Join(outputDir, "sections")
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}
	if maxItems <= 0 {
		return writeNodes(base, nodes, mdByID, []string{}, nil, limits)
	}
	remaining := maxItems
	return writeNodes(base, nodes, mdByID, []string{}, &remaining, limits)
}

func writeNodes(base string, nodes []menu.Node, mdByID map[string]string, pathParts []string, remaining *int, limits ChunkLimits) error {
	for _, node := range nodes {
		if remaining != nil && *remaining == 0 {
			return nil
		}
		part := slugify(node.Title)
		if part == "" {
			part = slugify(node.Anchor)
		}
		if part == "" {
			part = "section"
		}

		localPath := append(pathParts, part)
		if node.Anchor != "" {
			if md, ok := mdByID[node.Anchor]; ok && strings.TrimSpace(md) != "" {
				filePath := filepath.Join(append([]string{base}, localPath...)...)
				if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
					return err
				}
				if err := writeMarkdownFile(filePath, md, limits); err != nil {
					return err
				}
				if remaining != nil && *remaining > 0 {
					*remaining--
				}
			}
		}

		if len(node.Children) > 0 {
			if err := writeNodes(base, node.Children, mdByID, localPath, remaining, limits); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMarkdownFile(basePath string, md string, limits ChunkLimits) error {
	if !limits.Enabled() || !limits.exceeds(sizeOfString(md)) {
		return os.WriteFile(basePath+".md", []byte(md), 0600)
	}

	parts := splitMarkdownByHeadings(md, limits)
	if len(parts) == 0 {
		return os.WriteFile(basePath+".md", []byte(md), 0600)
	}

	partDir := basePath
	if err := os.MkdirAll(partDir, 0755); err != nil {
		return err
	}

	for i, part := range parts {
		partPath := filepath.Join(partDir, fmt.Sprintf("part-%03d.md", i+1))
		if err := os.WriteFile(partPath, []byte(part), 0600); err != nil {
			return err
		}
	}

	index := buildSplitIndex(firstHeadingLine(md), filepath.Base(basePath), len(parts))
	return os.WriteFile(basePath+".md", []byte(index), 0600)
}

func buildSplitIndex(heading string, partDir string, parts int) string {
	var b strings.Builder
	if heading != "" {
		b.WriteString(heading)
		b.WriteString("\n\n")
	}
	b.WriteString(fmt.Sprintf("Split into %d parts:\n\n", parts))
	for i := 1; i <= parts; i++ {
		b.WriteString(fmt.Sprintf("- %s/part-%03d.md\n", partDir, i))
	}
	return b.String()
}

func firstHeadingLine(md string) string {
	for _, line := range strings.Split(md, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			return line
		}
		break
	}
	return ""
}

func splitMarkdownByHeadings(md string, limits ChunkLimits) []string {
	md = strings.TrimSpace(md)
	if md == "" {
		return nil
	}
	if !limits.Enabled() || !limits.exceeds(sizeOfString(md)) {
		return []string{md}
	}

	prefix, body := splitHeadingPrefix(md)
	if limits.exceeds(sizeOfString(prefix)) {
		prefix = ""
	}

	blocks := splitOnSubheadings(body)
	writer := newChunkWriter(prefix, limits)
	for _, block := range blocks {
		writer.AddBlock(block)
	}
	return writer.Parts()
}

func splitHeadingPrefix(md string) (string, string) {
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			rest := strings.Join(lines[i+1:], "\n")
			rest = strings.TrimSpace(rest)
			return strings.TrimSpace(line) + "\n\n", rest
		}
		break
	}
	return "", md
}

func splitOnSubheadings(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	blocks := []string{}
	var cur strings.Builder
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		isHeading := strings.HasPrefix(trim, "### ")
		if strings.HasPrefix(trim, "#### ") || strings.HasPrefix(trim, "##### ") || strings.HasPrefix(trim, "###### ") {
			isHeading = true
		}
		if isHeading && cur.Len() > 0 {
			blocks = append(blocks, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	if cur.Len() > 0 {
		blocks = append(blocks, strings.TrimSpace(cur.String()))
	}
	return blocks
}

type chunkWriter struct {
	prefix string
	limits ChunkLimits
	cur    strings.Builder
	size   chunkSize
	parts  []string
}

func newChunkWriter(prefix string, limits ChunkLimits) *chunkWriter {
	w := &chunkWriter{
		prefix: prefix,
		limits: limits,
		size:   sizeOfString(prefix),
		parts:  []string{},
	}
	w.cur.WriteString(prefix)
	return w
}

func (w *chunkWriter) AddBlock(block string) {
	block = strings.TrimSpace(block)
	if block == "" {
		return
	}
	for _, sub := range w.expandSubBlocks(block) {
		w.addSubBlock(sub)
	}
}

func (w *chunkWriter) expandSubBlocks(block string) []string {
	if w.limits.exceeds(sizeOfString(block)) {
		return splitOnParagraphs(block)
	}
	return []string{block}
}

func (w *chunkWriter) addSubBlock(sub string) {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return
	}
	sep := ""
	if w.hasContentBeyondPrefix() {
		sep = "\n\n"
	}
	combined := w.size.add(sizeOfString(sep)).add(sizeOfString(sub))
	if w.hasContentBeyondPrefix() && w.limits.exceeds(combined) {
		w.flush()
		sep = ""
		combined = w.size.add(sizeOfString(sub))
	}
	if sep != "" {
		w.cur.WriteString(sep)
		w.size = w.size.add(sizeOfString(sep))
	}
	w.cur.WriteString(sub)
	w.size = combined
	if w.limits.exceeds(w.size) && w.hasContentBeyondPrefix() {
		w.flush()
	}
}

func (w *chunkWriter) hasContentBeyondPrefix() bool {
	return curHasContentBeyondPrefix(w.size, w.prefix)
}

func (w *chunkWriter) flush() {
	out := strings.TrimSpace(w.cur.String())
	if out != "" {
		w.parts = append(w.parts, out+"\n")
	}
	w.cur.Reset()
	w.cur.WriteString(w.prefix)
	w.size = sizeOfString(w.prefix)
}

func (w *chunkWriter) Parts() []string {
	out := strings.TrimSpace(w.cur.String())
	if out != "" {
		w.parts = append(w.parts, out+"\n")
	}
	return w.parts
}

func splitOnParagraphs(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	blocks := strings.Split(body, "\n\n")
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		out = append(out, b)
	}
	return out
}

func sizeOfString(s string) chunkSize {
	if s == "" {
		return chunkSize{}
	}
	chars := len([]rune(s))
	return chunkSize{
		bytes:  len(s),
		chars:  chars,
		tokens: estimateTokens(chars),
	}
}

func estimateTokens(chars int) int {
	if chars == 0 {
		return 0
	}
	return (chars + 3) / 4
}

func (s chunkSize) add(o chunkSize) chunkSize {
	return chunkSize{
		bytes:  s.bytes + o.bytes,
		chars:  s.chars + o.chars,
		tokens: s.tokens + o.tokens,
	}
}

func curHasContentBeyondPrefix(cur chunkSize, prefix string) bool {
	if strings.TrimSpace(prefix) == "" {
		return cur.bytes > 0
	}
	prefixSize := sizeOfString(prefix)
	return cur.bytes > prefixSize.bytes || cur.chars > prefixSize.chars || cur.tokens > prefixSize.tokens
}

func bundleParts(parts []string, limits ChunkLimits) []string {
	bundles := []string{}
	var cur strings.Builder
	curSize := chunkSize{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.HasSuffix(part, "\n") {
			part += "\n"
		}
		partSize := sizeOfString(part)
		if curSize.bytes > 0 && limits.exceeds(curSize.add(partSize)) {
			bundles = append(bundles, strings.TrimSpace(cur.String())+"\n")
			cur.Reset()
			curSize = chunkSize{}
		}
		if curSize.bytes == 0 && limits.exceeds(partSize) {
			bundles = append(bundles, strings.TrimSpace(part)+"\n")
			continue
		}
		cur.WriteString(part)
		curSize = curSize.add(partSize)
		if limits.exceeds(curSize) {
			bundles = append(bundles, strings.TrimSpace(cur.String())+"\n")
			cur.Reset()
			curSize = chunkSize{}
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		bundles = append(bundles, strings.TrimSpace(cur.String())+"\n")
	}
	return bundles
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "|", "")
	s = strings.Join(strings.Fields(s), "-")
	return s
}
