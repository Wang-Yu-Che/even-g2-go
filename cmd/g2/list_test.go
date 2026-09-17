package main

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

type listDisplayCall struct {
	kind    string
	name    string
	content string
	rows    []string
}

type recordingListDisplay struct {
	calls []listDisplayCall
}

func (d *recordingListDisplay) DisplayList(_ context.Context, name string, rows []string) error {
	d.calls = append(d.calls, listDisplayCall{kind: "list", name: name, rows: append([]string(nil), rows...)})
	return nil
}

func (d *recordingListDisplay) ShowText(_ context.Context, name, content string) error {
	d.calls = append(d.calls, listDisplayCall{kind: "text", name: name, content: content})
	return nil
}

func TestListNavigationOpensDetailAndReturns(t *testing.T) {
	navigation := listNavigation{rows: []string{"第一项", "第二项完整内容"}}
	display := &recordingListDisplay{}
	now := time.Unix(100, 0)

	if err := navigation.handle(context.Background(), display, protocol.EvenHubEvent{
		Kind: protocol.EvenHubEventList, Name: "hud", ItemIndex: 1, Type: protocol.EvenHubEventClick,
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := navigation.handle(context.Background(), display, protocol.EvenHubEvent{
		Kind: protocol.EvenHubEventSystem, Type: protocol.EvenHubEventDoubleClick,
	}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	want := []listDisplayCall{
		{kind: "text", name: "hud", content: "第二项完整内容\n\n双击返回列表"},
		{kind: "list", name: "hud", rows: []string{"第一项", "第二项完整内容"}},
	}
	if !reflect.DeepEqual(display.calls, want) {
		t.Fatalf("calls = %#v, want %#v", display.calls, want)
	}
}

func TestListNavigationIgnoresDuplicateAndInvalidClicks(t *testing.T) {
	navigation := listNavigation{rows: []string{"第一项"}}
	display := &recordingListDisplay{}
	now := time.Unix(100, 0)

	event := protocol.EvenHubEvent{Kind: protocol.EvenHubEventList, Name: "hud", Type: protocol.EvenHubEventClick}
	if err := navigation.handle(context.Background(), display, event, now); err != nil {
		t.Fatal(err)
	}
	if err := navigation.handle(context.Background(), display, event, now.Add(100*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := navigation.handle(context.Background(), display, protocol.EvenHubEvent{
		Kind: protocol.EvenHubEventList, Name: "other", ItemIndex: 0, Type: protocol.EvenHubEventClick,
	}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	if len(display.calls) != 1 || display.calls[0].kind != "text" {
		t.Fatalf("calls = %#v, want one text call", display.calls)
	}
}
