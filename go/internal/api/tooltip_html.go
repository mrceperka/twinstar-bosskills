package api

import (
	"bytes"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var tooltipElements = map[string]bool{
	"a": true, "b": true, "br": true, "div": true, "em": true,
	"i": true, "img": true, "li": true, "ol": true, "p": true, "small": true,
	"span": true, "strong": true, "table": true, "tbody": true,
	"td": true, "th": true, "thead": true, "tr": true, "u": true,
	"ul": true,
}

var (
	tooltipColorRE      = regexp.MustCompile(`(?i)^(?:#[0-9a-f]{3,8}|rgba?\([0-9.,% ]+\)|[a-z]+)$`)
	tooltipLengthRE     = regexp.MustCompile(`(?i)^(?:0|[0-9]+(?:\.[0-9]+)?(?:px|rem|em|%))(?:\s+(?:0|[0-9]+(?:\.[0-9]+)?(?:px|rem|em|%))){0,3}$`)
	tooltipBackgroundRE = regexp.MustCompile(`(?i)^url\((?:"|')?(https://twinstar-api\.twinstar-wow\.com/img/socket/[a-z0-9_.-]+)(?:"|')?\)\s+no-repeat\s+left\s+center$`)
)

var tooltipDropWithContents = map[string]bool{
	"button": true, "embed": true, "form": true, "iframe": true,
	"input": true, "math": true, "object": true, "script": true,
	"select": true, "style": true, "svg": true, "template": true,
	"textarea": true,
}

// SanitizeTooltipHTML keeps the small formatting subset needed by item
// tooltips and removes active content, inline CSS, event handlers, and unsafe
// URLs. Tooltip markup is supplied by an upstream service and must not cross
// into templ.Raw or innerHTML unsanitized.
func SanitizeTooltipHTML(input string) string {
	root := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(input), root)
	if err != nil {
		return ""
	}
	for _, node := range nodes {
		root.AppendChild(node)
	}
	sanitizeTooltipChildren(root)

	var out bytes.Buffer
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		_ = html.Render(&out, node)
	}
	return out.String()
}

func sanitizeTooltipChildren(parent *html.Node) {
	for node := parent.FirstChild; node != nil; {
		next := node.NextSibling
		if node.Type == html.ElementNode {
			name := strings.ToLower(node.Data)
			switch {
			case tooltipDropWithContents[name]:
				parent.RemoveChild(node)
				node = next
				continue
			case !tooltipElements[name]:
				sanitizeTooltipChildren(node)
				for child := node.FirstChild; child != nil; {
					childNext := child.NextSibling
					node.RemoveChild(child)
					parent.InsertBefore(child, node)
					child = childNext
				}
				parent.RemoveChild(node)
				node = next
				continue
			default:
				node.Attr = sanitizeTooltipAttrs(name, node.Attr)
			}
		}
		sanitizeTooltipChildren(node)
		node = next
	}
}

func sanitizeTooltipAttrs(element string, attrs []html.Attribute) []html.Attribute {
	out := make([]html.Attribute, 0, len(attrs))
	for _, attr := range attrs {
		key := strings.ToLower(attr.Key)
		switch key {
		case "class", "title", "aria-label", "alt", "colspan", "rowspan":
			out = append(out, html.Attribute{Key: key, Val: attr.Val})
		case "width", "height":
			if attr.Val == "100%" || tooltipLengthRE.MatchString(strings.TrimSpace(attr.Val)) {
				out = append(out, html.Attribute{Key: key, Val: attr.Val})
			}
		case "href":
			if element == "a" && safeTooltipURL(attr.Val, false) {
				out = append(out, html.Attribute{Key: key, Val: attr.Val})
			}
		case "src":
			if element == "img" && safeTooltipURL(attr.Val, true) {
				out = append(out, html.Attribute{Key: key, Val: attr.Val})
			}
		case "style":
			if style := sanitizeTooltipStyle(attr.Val); style != "" {
				out = append(out, html.Attribute{Key: key, Val: style})
			}
		}
	}
	return out
}

// sanitizeTooltipStyle retains the small CSS vocabulary emitted by the
// Twinstar tooltip API. In particular, socket icons are supplied as a
// background image rather than an img element. Everything else is discarded.
func sanitizeTooltipStyle(raw string) string {
	var safe []string
	for _, declaration := range strings.Split(raw, ";") {
		parts := strings.SplitN(declaration, ":", 2)
		if len(parts) != 2 {
			continue
		}
		property := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		valid := false
		switch property {
		case "color", "background-color", "border-color":
			valid = tooltipColorRE.MatchString(value)
		case "padding", "padding-left", "background-size", "border-spacing":
			valid = tooltipLengthRE.MatchString(value)
		case "text-align":
			valid = value == "left" || value == "right" || value == "center"
		case "background":
			valid = tooltipBackgroundRE.MatchString(value)
		}
		if valid {
			safe = append(safe, property+": "+value)
		}
	}
	return strings.Join(safe, "; ")
}

func safeTooltipURL(raw string, image bool) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.IsAbs() {
		return u.Scheme == "https" || (!image && u.Scheme == "http")
	}
	return strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "./") || strings.HasPrefix(u.Path, "../") || u.Path == ""
}
