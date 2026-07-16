package report

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var sectionMarkdown = goldmark.New()

type markdownSection struct {
	heading      *ast.Heading
	headingStart int
	headingEnd   int
	sectionEnd   int
	body         []ast.Node
}

// semanticMarkdownComponents promotes only an H2/H3 section whose entire body
// is one explicit ordered list. Every other byte remains in an article range.
// The resulting ranges partition src in source order.
func semanticMarkdownComponents(src []byte) []Component {
	doc := sectionMarkdown.Parser().Parse(text.NewReader(src))
	sections := markdownSections(doc, src)

	type timelineRange struct {
		start     int
		end       int
		component Component
	}
	var timelines []timelineRange
	for _, section := range sections {
		if len(section.body) != 1 {
			continue
		}
		list, ok := section.body[0].(*ast.List)
		if !ok || !list.IsOrdered() || list.ChildCount() < 2 {
			continue
		}
		listStart, listEnd, ok := SourceRangeForNode(list, src)
		if !ok {
			continue
		}
		items := make([]SourceRef, 0, list.ChildCount())
		valid := true
		for item := list.FirstChild(); item != nil; item = item.NextSibling() {
			start, end, ok := SourceRangeForNode(item, src)
			if !ok {
				valid = false
				break
			}
			items = append(items, newSourceRef(src, "list-item", start, end))
		}
		if !valid || len(items) < 2 {
			continue
		}
		timeline := &TimelineData{
			Section: newSourceRef(src, "section", section.headingStart, section.sectionEnd),
			Heading: newSourceRef(src, "heading", section.headingStart, section.headingEnd),
			List:    newSourceRef(src, "ordered-list", listStart, listEnd),
			Items:   items,
		}
		timelines = append(timelines, timelineRange{
			start: section.headingStart,
			end:   section.sectionEnd,
			component: Component{
				Type: ComponentTimeline, Source: "input", Title: "Timeline",
				Options: map[string]string{}, Timeline: timeline,
			},
		})
	}
	if len(timelines) == 0 {
		return nil
	}

	components := make([]Component, 0, len(timelines)*2+1)
	cursor := 0
	for _, timeline := range timelines {
		if cursor < timeline.start {
			components = append(components, sourceArticle(src, cursor, timeline.start))
		}
		components = append(components, timeline.component)
		cursor = timeline.end
	}
	if cursor < len(src) {
		components = append(components, sourceArticle(src, cursor, len(src)))
	}
	return components
}

func sourceArticle(src []byte, start, end int) Component {
	return Component{
		Type: ComponentArticle, Source: "input", Title: "Article",
		Options: map[string]string{}, Article: &ArticleData{Range: newSourceRef(src, "article", start, end)},
	}
}

func markdownSections(doc ast.Node, src []byte) []markdownSection {
	var sections []markdownSection
	var current *markdownSection
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		heading, isHeading := node.(*ast.Heading)
		if isHeading && (heading.Level == 2 || heading.Level == 3) {
			start, end, ok := SourceRangeForNode(heading, src)
			if !ok {
				current = nil
				continue
			}
			if current != nil {
				current.sectionEnd = start
			}
			sections = append(sections, markdownSection{heading: heading, headingStart: start, headingEnd: end, sectionEnd: len(src)})
			current = &sections[len(sections)-1]
			continue
		}
		if current != nil {
			current.body = append(current.body, node)
		}
	}
	return sections
}

// SourceRangeForNode returns the full line-owned byte range of a Goldmark
// block node, including Markdown markers omitted from AST line segments.
func SourceRangeForNode(node ast.Node, src []byte) (int, int, bool) {
	start, end := len(src), -1
	_ = ast.Walk(node, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || current.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		lines := current.Lines()
		for i := 0; i < lines.Len(); i++ {
			segment := lines.At(i)
			if segment.Start < start {
				start = segment.Start
			}
			if segment.Stop > end {
				end = segment.Stop
			}
		}
		return ast.WalkContinue, nil
	})
	if end < 0 || start >= len(src) {
		return 0, 0, false
	}
	start = lineStart(src, start)
	end = lineEnd(src, end)
	return start, end, start < end
}

func lineStart(src []byte, offset int) int {
	for offset > 0 && src[offset-1] != '\n' {
		offset--
	}
	return offset
}

func lineEnd(src []byte, offset int) int {
	if offset > len(src) {
		offset = len(src)
	}
	for offset < len(src) && src[offset] != '\n' {
		offset++
	}
	if offset < len(src) {
		offset++
	}
	return offset
}

func newSourceRef(src []byte, kind string, start, end int) SourceRef {
	digest := sha256.Sum256(src[start:end])
	idInput := fmt.Sprintf("%s:%d:%d:%x", kind, start, end, digest)
	id := sha256.Sum256([]byte(idInput))
	return SourceRef{
		ID:     kind + "-" + hex.EncodeToString(id[:6]),
		Kind:   kind,
		Start:  start,
		End:    end,
		Digest: hex.EncodeToString(digest[:]),
	}
}

// ValidateComponentSources verifies that semantic component ranges still own
// the exact source bytes they were planned from and partition the full input.
func ValidateComponentSources(src []byte, components []Component) error {
	if err := validateSemanticOwnership(components); err != nil {
		return err
	}
	hasSemantic := false
	expectedStart := 0
	seen := map[string]struct{}{}
	for i, component := range components {
		if err := validateComponentData(component); err != nil {
			return fmt.Errorf("component %d: %w", i, err)
		}
		var top *SourceRef
		switch {
		case component.Article != nil:
			hasSemantic = true
			top = &component.Article.Range
		case component.Timeline != nil:
			hasSemantic = true
			top = &component.Timeline.Section
		default:
			continue
		}
		if top.Start != expectedStart {
			return fmt.Errorf("component %d does not own the next source byte", i)
		}
		if err := validateSourceBytes(src, *top); err != nil {
			return fmt.Errorf("component %d: %w", i, err)
		}
		if _, exists := seen[top.ID]; exists {
			return fmt.Errorf("component %d duplicates source ref %q", i, top.ID)
		}
		seen[top.ID] = struct{}{}
		expectedStart = top.End
		if component.Timeline != nil {
			t := component.Timeline
			refs := append([]SourceRef{t.Heading, t.List}, t.Items...)
			for _, ref := range refs {
				if _, exists := seen[ref.ID]; exists {
					return fmt.Errorf("component %d duplicates source ref %q", i, ref.ID)
				}
				seen[ref.ID] = struct{}{}
				if err := validateSourceBytes(src, ref); err != nil {
					return fmt.Errorf("component %d: %w", i, err)
				}
			}
		}
	}
	if hasSemantic && expectedStart != len(src) {
		return fmt.Errorf("semantic components do not own the complete source")
	}
	return nil
}

func validateSourceBytes(src []byte, ref SourceRef) error {
	if ref.Start < 0 || ref.End <= ref.Start || ref.End > len(src) {
		return fmt.Errorf("source ref %q is out of bounds", ref.ID)
	}
	digest := sha256.Sum256(src[ref.Start:ref.End])
	if hex.EncodeToString(digest[:]) != ref.Digest {
		return fmt.Errorf("source ref %q is stale", ref.ID)
	}
	return nil
}
