package authctx

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app"
)

const (
	userIDKey   = "userID"
	tokenExpKey = "tokenExp"
	tokenIDKey  = "tokenID"
)

func SetUserID(c *app.RequestContext, id int64) {
	c.Set(userIDKey, id)
}

func UserID(c *app.RequestContext) (int64, bool) {
	v, ok := c.Get(userIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}

func SetTokenExp(c *app.RequestContext, exp time.Time) {
	c.Set(tokenExpKey, exp)
}

func TokenExp(c *app.RequestContext) (time.Time, bool) {
	v, ok := c.Get(tokenExpKey)
	if !ok {
		return time.Time{}, false
	}
	exp, ok := v.(time.Time)
	return exp, ok
}

func SetTokenID(c *app.RequestContext, id string) {
	c.Set(tokenIDKey, id)
}

func TokenID(c *app.RequestContext) (string, bool) {
	v, ok := c.Get(tokenIDKey)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}
