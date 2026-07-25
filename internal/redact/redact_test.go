package redact

import (
	"strings"
	"testing"

	"github.com/Vliysl/roblox-studio-doctor/internal/parse"
)

func TestEventsDropsUnlisted(t *testing.T) {
	in := []parse.Event{
		{Channel: "TeamCreateManager", Message: "Disconnected"},
		{Channel: "Output", Message: "print('my secret place name')"},
		{Channel: "SomeFutureChannel", Message: "whatever"},
	}
	got := Events(in)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Channel != "TeamCreateManager" {
		t.Errorf("kept %q", got[0].Channel)
	}
}

func TestTextStableTokens(t *testing.T) {
	in := "GameId=10553200977 and again 10553200977 but 90578119 differs"
	got := Text(in)
	if strings.Contains(got, "10553200977") {
		t.Fatalf("raw id survived: %q", got)
	}
	if strings.Contains(got, "90578119") {
		t.Fatalf("raw id survived: %q", got)
	}

	first := strings.Index(got, "id_")
	if first < 0 {
		t.Fatalf("no pseudonym emitted: %q", got)
	}
	tok := got[first : first+9]
	if strings.Count(got, tok) != 2 {
		t.Errorf("unstable pseudonym in %q", got)
	}
}

func TestTextKeepsShortNumbers(t *testing.T) {
	in := "Disconnected due to DisconnectClientInitiated (285). LostConnection = false"
	got := Text(in)
	if !strings.Contains(got, "285") {
		t.Errorf("error code was redacted: %q", got)
	}
	if !strings.Contains(got, "LostConnection = false") {
		t.Errorf("payload damaged: %q", got)
	}
}

func TestTextRedactsPaths(t *testing.T) {
	for _, in := range []string{
		`C:\users\realname\Documents\MyGame.rbxl`,
		`/home/realname/Projects/MyGame.rbxl`,
	} {
		got := Text(in)
		if strings.Contains(got, "realname") {
			t.Errorf("username survived in %q -> %q", in, got)
		}
	}
}

func TestEventsRedactsMessages(t *testing.T) {
	in := []parse.Event{{Channel: "TeamCreateManager", Message: "place 10553200977 dropped"}}
	got := Events(in)
	if strings.Contains(got[0].Message, "10553200977") {
		t.Errorf("id survived in event message: %q", got[0].Message)
	}
}
