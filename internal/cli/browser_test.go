package cli

import (
	"reflect"
	"testing"
)

func TestBrowserCommand(t *testing.T) {
	const url = "http://127.0.0.1:8080"
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{"darwin", "open", []string{url}},
		{"windows", "rundll32", []string{"url.dll,FileProtocolHandler", url}},
		{"linux", "xdg-open", []string{url}},
	}
	for _, tt := range tests {
		t.Run(tt.goos, func(t *testing.T) {
			name, args := browserCommand(tt.goos, url)
			if name != tt.wantName || !reflect.DeepEqual(args, tt.wantArgs) {
				t.Errorf("browserCommand(%q) = %q %v, want %q %v", tt.goos, name, args, tt.wantName, tt.wantArgs)
			}
		})
	}
}
