package ui

// detailView shows a single object's YAML in a pager with theme-aware syntax
// highlighting. It inherits scroll, wrap toggle, regex filter, and whole-line
// selection/copy from the embedded pager.
type detailView struct {
	pager
}

func newDetailView(th Theme) detailView {
	d := detailView{pager: newPager(th)}
	d.follow = false // static content: load at the top, don't chase a tail
	return d
}

// setMessage shows plain (unhighlighted) text such as "loading…" or an error.
func (d *detailView) setMessage(title, body string) {
	d.title = title
	d.clearFilter()
	d.SetContent(body)
}

// setYAML renders highlighted YAML.
func (d *detailView) setYAML(title, yaml string) {
	d.title = title
	d.clearFilter()
	d.SetContent(highlightYAML(yaml, d.th))
}

func (d detailView) View() string {
	right, ok := d.selStatus()
	if !ok {
		right = d.th.Dim.Render(scrollPercent(d.vp.ScrollPercent()))
	}
	return d.view(right)
}

func scrollPercent(f float64) string {
	return itoa(clamp(int(f*100), 0, 100)) + "%"
}
