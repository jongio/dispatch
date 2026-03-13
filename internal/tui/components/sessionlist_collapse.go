package components

// CollapseAll sets all folder groups to collapsed state and rebuilds
// the visible item list. Used by the screenshot generator.
func (s *SessionList) CollapseAll() {
	for k := range s.expanded {
		delete(s.expanded, k)
	}
	s.rebuildVisible()
}
