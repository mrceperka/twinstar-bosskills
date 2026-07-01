package changelog

// paragraphClass picks paragraph spacing based on position and whether the
// box has a title tab. When there is a tab, the first paragraph needs a top
// margin to clear the tab; when there isn't, the paragraph should sit with
// normal vertical padding inside the box.
func paragraphClass(index int, hasTitle bool) string {
	if index == 0 {
		if hasTitle {
			return "text-sm text-bk-text mt-3 mb-2"
		}
		return "text-sm text-bk-text py-2"
	}
	return "text-sm text-bk-text mt-2 mb-2"
}
