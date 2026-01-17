package tui

import (
	"reflect"
	"testing"
)

func TestStylesPalette(t *testing.T) {
	styles := NewStyles()
	if styles.Primary == "" {
		t.Fatal("primary color missing")
	}
}

func TestStylesHasNoTabLabel(t *testing.T) {
	styles := NewStyles()
	typeOf := reflect.TypeOf(styles)
	if _, ok := typeOf.FieldByName("TabLabel"); ok {
		t.Fatal("tab label style should not exist")
	}
}
