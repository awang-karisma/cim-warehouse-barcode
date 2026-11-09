package accurate

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"

	nethtml "golang.org/x/net/html"
)

// FormPayload contains the action URL, method, and key-value inputs of an HTML form.
type FormPayload struct {
	Action string
	Method string
	Fields map[string]string
}

// ExtractForm parses an HTML document using net/html and extracts the form matching an optional name or input field name.
// HTML entities in attribute values and form actions are unescaped.
func ExtractForm(r io.Reader, requiredFieldName string) (*FormPayload, error) {
	doc, err := nethtml.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var found *FormPayload

	var traverse func(*nethtml.Node)
	traverse = func(n *nethtml.Node) {
		if found != nil {
			return
		}

		if n.Type == nethtml.ElementNode && strings.EqualFold(n.Data, "form") {
			payload := &FormPayload{
				Fields: make(map[string]string),
				Method: "POST",
			}

			for _, attr := range n.Attr {
				key := strings.ToLower(attr.Key)
				if key == "action" {
					payload.Action = html.UnescapeString(attr.Val)
				} else if key == "method" {
					payload.Method = strings.ToUpper(html.UnescapeString(attr.Val))
				}
			}

			// Gather inputs inside this form
			var gatherInputs func(*nethtml.Node)
			gatherInputs = func(curr *nethtml.Node) {
				if curr.Type == nethtml.ElementNode && strings.EqualFold(curr.Data, "input") {
					var name, val string
					for _, attr := range curr.Attr {
						k := strings.ToLower(attr.Key)
						if k == "name" {
							name = html.UnescapeString(attr.Val)
						} else if k == "value" {
							val = html.UnescapeString(attr.Val)
						}
					}
					if name != "" {
						payload.Fields[name] = val
					}
				}
				for c := curr.FirstChild; c != nil; c = c.NextSibling {
					gatherInputs(c)
				}
			}

			gatherInputs(n)

			if requiredFieldName == "" {
				found = payload
				return
			}
			if _, ok := payload.Fields[requiredFieldName]; ok {
				found = payload
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	if found == nil {
		return nil, errors.New("form not found in HTML")
	}
	return found, nil
}

// ExtractContentDataFromScripts parses HTML script elements to locate `var contentData = { ... };`.
func ExtractContentDataFromScripts(r io.Reader) (map[string]any, error) {
	doc, err := nethtml.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var jsonStr string

	var traverse func(*nethtml.Node)
	traverse = func(n *nethtml.Node) {
		if jsonStr != "" {
			return
		}

		if n.Type == nethtml.ElementNode && strings.EqualFold(n.Data, "script") {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == nethtml.TextNode {
					text := c.Data
					if idx := strings.Index(text, "contentData"); idx != -1 {
						// Look for = after contentData
						eqIdx := strings.Index(text[idx:], "=")
						if eqIdx != -1 {
							start := idx + eqIdx + 1
							rest := strings.TrimSpace(text[start:])
							if strings.HasPrefix(rest, "{") {
								// Find matching closing brace
								depth := 0
								inString := false
								var strChar rune
								var escaped bool
								endIdx := -1

								for i, ch := range rest {
									if inString {
										if escaped {
											escaped = false
										} else if ch == '\\' {
											escaped = true
										} else if ch == strChar {
											inString = false
										}
										continue
									}

									if ch == '"' || ch == '\'' {
										inString = true
										strChar = ch
									} else if ch == '{' {
										depth++
									} else if ch == '}' {
										depth--
										if depth == 0 {
											endIdx = i + 1
											break
										}
									}
								}

								if endIdx != -1 {
									jsonStr = rest[:endIdx]
									return
								}
							}
						}
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	if jsonStr == "" {
		return nil, errors.New("contentData not found in HTML scripts")
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contentData JSON: %w", err)
	}

	return result, nil
}
