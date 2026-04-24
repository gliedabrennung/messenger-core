package authctx

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
)

func TestSetAndGetUserID(t *testing.T) {
	c := app.NewContext(16)
	
	id := int64(456)
	SetUserID(c, id)
	
	gotID, ok := UserID(c)
	if !ok {
		t.Fatal("expected to find user ID in context")
	}
	if gotID != id {
		t.Errorf("expected %d, got %d", id, gotID)
	}
}

func TestUserID_NotFound(t *testing.T) {
	c := app.NewContext(16)
	_, ok := UserID(c)
	if ok {
		t.Error("expected ok to be false for missing user ID")
	}
}
