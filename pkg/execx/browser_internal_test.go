package execx

import (
	"reflect"
	"testing"
)

func TestBrowserOpeners(t *testing.T) {
	tests := []struct {
		goos string
		want [][]string
	}{
		{"darwin", [][]string{
			{"open", "-a", "Google Chrome", "report.html"},
			{"open", "report.html"},
		}},
		{"linux", [][]string{{"xdg-open", "report.html"}}},
		{"windows", [][]string{{"rundll32", "url.dll,FileProtocolHandler", "report.html"}}},
		{"plan9", nil},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			if got := browserOpeners(tt.goos, "report.html"); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("browserOpeners(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}
