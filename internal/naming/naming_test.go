package naming

import "testing"

// taken makes an exists func for a fixed set of names.
func taken(names ...string) func(string) bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return func(name string) bool { return set[name] }
}

func TestAFreeNameIsKeptAsItIs(t *testing.T) {
	if got := Free("report.pdf", taken()); got != "report.pdf" {
		t.Errorf("got %q", got)
	}
}

func TestAClashIsNumberedBeforeTheExtension(t *testing.T) {
	cases := []struct {
		name  string
		taken []string
		want  string
	}{
		{"report.pdf", []string{"report.pdf"}, "report 2.pdf"},
		{"report.pdf", []string{"report.pdf", "report 2.pdf"}, "report 3.pdf"},
		{"photos.tar.gz", []string{"photos.tar.gz"}, "photos.tar 2.gz"},
		{"README", []string{"README"}, "README 2"},
		{".env", []string{".env"}, ".env 2"},
	}
	for _, c := range cases {
		if got := Free(c.name, taken(c.taken...)); got != c.want {
			t.Errorf("%s with %v taken: got %q, want %q", c.name, c.taken, got, c.want)
		}
	}
}
